package envoy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/utils/strutil"
	"gopkg.in/yaml.v3"
)

type YAMLParser struct {
	cfg *Configuration
}

func (p *YAMLParser) solveCommands(path string, data any) (any, error) {
	switch val := data.(type) {
	case map[string]any:
		return p.solveCommandsInMap(path, val)
	case []any:
		return p.solveCommandsInSlice(path, val)
	}
	return data, nil
}

func (p *YAMLParser) solveCommandsInMap(path string, data map[string]any) (map[string]any, error) {
	var err error
	for k, v := range data {
		nextPath := getNextPath(path, k)
		cmd, ok := p.isCommand(v)
		if !ok {
			data[k], err = p.solveCommands(nextPath, v)
			if err != nil {
				return nil, err
			}
			continue
		}
		cmdResult, err := p.runCommand(cmd, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: ran command %s: %w", nextPath, cmd, err)
		}
		cmdResult, err = p.solveCommands(nextPath, cmdResult)
		if err != nil {
			return nil, err
		}
		data[k] = cmdResult
	}

	newData := make(map[string]map[string]any)
	for k, v := range data {
		nextPath := getNextPath(path, k)
		cmd, ok := p.isCommand(k)
		if !ok {
			continue
		}
		cmdResult, err := p.runCommand(cmd, v)
		if err != nil {
			return nil, fmt.Errorf("%s: run command %s: %w", nextPath, cmd, err)
		}
		if cmdResult == nil {
			delete(data, k)
			continue
		}
		m, ok := cmdResult.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: command %s want map result, but got %v", path, cmd, cmdResult)
		}
		m, err = p.solveCommandsInMap(nextPath, m)
		if err != nil {
			return nil, err
		}
		newData[k] = m
		delete(data, k)
	}

	for _, cmdResult := range newData {
		easy.MergeMapsTo(data, cmdResult)
	}
	return data, nil
}

func (p *YAMLParser) solveCommandsInSlice(path string, slice []any) (any, error) {
	isSlice := func(a any) bool {
		_, ok := a.([]any)
		return ok
	}

	var err error
	var i int
	for i = 0; i < len(slice); {
		nextPath := getNextPath(path, fmt.Sprintf("[%d]", i))
		val := slice[i]
		cmd, ok := p.isCommand(val)
		if !ok {
			slice[i], err = p.solveCommands(nextPath, val)
			if err != nil {
				return nil, err
			}
			i++
			continue
		}
		cmdResult, err := p.runCommand(cmd, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: run command %s: %w", nextPath, cmd, err)
		}
		cmdResult, err = p.solveCommands(nextPath, cmdResult)
		if err != nil {
			return nil, err
		}
		if cmdResult == nil {
			if len(slice) > i+1 {
				copy(slice[i:], slice[i+1:])
			}
			slice = slice[:len(slice)-1]
			continue
		}
		if !isSlice(cmdResult) {
			slice[i] = cmdResult
			i++
			continue
		}

		// Copy cmdResult elements into slice.
		s := cmdResult.([]any)
		newSlice := easy.Copy(slice[:i], cap(slice)+len(s))
		newSlice = append(newSlice, s...)
		newSlice = append(newSlice, s[i:]...)
		slice = newSlice
		i += len(s)
	}
	return slice, nil
}

func (p *YAMLParser) executeTemplate(s string, data any) (string, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("").Parse(s)
	if err != nil {
		return "", fmt.Errorf("cannot parse template (%s): %w", limit100(s), err)
	}
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("cannot execute template (%s): %w", limit100(s), err)
	}
	return buf.String(), nil
}

func (p *YAMLParser) parseYAML(s string, data ...any) (any, error) {
	var err error
	if len(data) > 0 {
		s, err = p.executeTemplate(s, data[0])
		if err != nil {
			return nil, err
		}
	}

	s = strings.TrimSpace(s)

	var dst any
	err = yaml.Unmarshal([]byte(s), &dst)
	if err != nil {
		return nil, fmt.Errorf("cannot parse yaml: %v", err)
	}
	return dst, nil
}

func getNextPath(path, next string) string {
	sep := "."
	isSliceIndex := len(next) > 2 && next[0] == '[' && next[len(next)-1] == ']' && strutil.IsASCIIDigit(next[1:len(next)-1])
	if isSliceIndex {
		sep = ""
	}
	if path == "" {
		return next
	}
	if next == "" {
		return path
	}
	return path + sep + next
}

func limit100(s string) string {
	if utf8.RuneCountInString(s) <= 100 {
		return s
	}
	r := []rune(s)
	return string(r[:97]) + "..."
}
