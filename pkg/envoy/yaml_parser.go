package envoy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/jxskiss/gopkg/v2/easy"
	"gopkg.in/yaml.v3"
)

type YAMLParser struct {
	cfg *Configuration
}

func (p *YAMLParser) solveCommands(data any) any {
	switch val := data.(type) {
	case map[string]any:
		return p.solveCommandsInMap(val)
	case []any:
		return p.solveCommandsInSlice(val)
	}
	return data
}

func (p *YAMLParser) solveCommandsInMap(data map[string]any) map[string]any {
	for k, v := range data {
		cmd, ok := p.isCommand(v)
		if !ok {
			data[k] = p.solveCommands(v)
			continue
		}
		cmdResult := p.runCommand(cmd, nil)
		cmdResult = p.solveCommands(cmdResult)
		data[k] = cmdResult
	}

	newData := make(map[string]map[string]any)
	for k, v := range data {
		cmd, ok := p.isCommand(k)
		if !ok {
			continue
		}
		cmdResult := p.runCommand(cmd, v)
		if cmdResult == nil {
			delete(data, k)
			continue
		}
		m, ok := cmdResult.(map[string]any)
		if !ok {
			panic(fmt.Sprintf("command %s want map result, but got %v", cmd, cmdResult))
		}
		m = p.solveCommandsInMap(m)
		newData[k] = m
		delete(data, k)
	}

	for _, cmdResult := range newData {
		easy.MergeMapsTo(data, cmdResult)
	}
	return data
}

func (p *YAMLParser) solveCommandsInSlice(slice []any) any {
	isSlice := func(a any) bool {
		_, ok := a.([]any)
		return ok
	}

	var i int
	for i = 0; i < len(slice); {
		val := slice[i]
		cmd, ok := p.isCommand(val)
		if !ok {
			slice[i] = p.solveCommands(val)
			i++
			continue
		}
		cmdResult := p.runCommand(cmd, nil)
		cmdResult = p.solveCommands(cmdResult)
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
	return slice
}

func (p *YAMLParser) executeTemplate(s string, data any) string {
	var buf bytes.Buffer
	tmpl, err := template.New("").Parse(s)
	if err != nil {
		panic(fmt.Sprintf("cannot parse template: %v", err))
	}
	err = tmpl.Execute(&buf, data)
	if err != nil {
		panic(fmt.Sprintf("cannot execute template: %v", err))
	}
	return buf.String()
}

func (p *YAMLParser) parseYAML(s string, data ...any) any {
	if len(data) > 0 {
		s = p.executeTemplate(s, data[0])
	}

	s = strings.TrimSpace(s)

	var dst any
	err := yaml.Unmarshal([]byte(s), &dst)
	if err != nil {
		panic(fmt.Sprintf("cannot parse yaml: %v", err))
	}
	return dst
}
