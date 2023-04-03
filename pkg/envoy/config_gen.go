package envoy

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/v2/easy"
	"gopkg.in/yaml.v3"
)

const bootstrapTpl = `
"!@@ envoy_node": {}
"!@@ envoy_admin": {}

dynamic_resources:
  lds_config:
    resource_api_version: V3
    path_config_source:
      path: "{{ .OutputPath }}/listeners.yaml"
  cds_config:
    resource_api_version: V3
    path_config_source:
      path: "{{ .OutputPath }}/clusters.yaml"

static_resources:
  clusters:
    - "!@@ sds_cluster"
`

func NewConfigGenerator(cfg *Configuration) *ConfigGenerator {
	gen := &ConfigGenerator{
		cfg: cfg,
	}
	return gen
}

type ConfigGenerator struct {
	cfg *Configuration
}

func (p *ConfigGenerator) Generate() error {
	if err := p.genBootstrapConfig(); err != nil {
		return err
	}
	if err := p.genListenersConfig(); err != nil {
		return err
	}
	if err := p.genClustersConfig(); err != nil {
		return err
	}
	return nil
}

func (p *ConfigGenerator) BootstrapConfigFile() string {
	return filepath.Join(p.cfg.OutputPath(), "bootstrap.yaml")
}

func (p *ConfigGenerator) genBootstrapConfig() error {
	parser := &YAMLParser{
		cfg: p.cfg,
	}
	yamlData := parser.parseYAML(bootstrapTpl, p.cfg)
	yamlData = parser.solveCommands(yamlData)

	outFile := p.BootstrapConfigFile()
	return p.writeYaml(outFile, yamlData, "")
}

func (p *ConfigGenerator) genListenersConfig() error {
	parser := &YAMLParser{
		cfg: p.cfg,
	}

	header := "# This file is auto generated, do not edit.\n\nresources:\n\n"

	tmplFile := filepath.Join(p.cfg.confDir, "listeners.yaml")
	yamlText, err := os.ReadFile(tmplFile)
	if err != nil {
		return errors.AddStack(err)
	}
	yamlData := parser.parseYAML(string(yamlText), p.cfg)
	yamlData = parser.solveCommands(yamlData)

	outFile := filepath.Join(p.cfg.OutputPath(), "listeners.yaml")
	return p.writeYaml(outFile, yamlData, header)
}

func (p *ConfigGenerator) genClustersConfig() error {
	parser := &YAMLParser{
		cfg: p.cfg,
	}

	header := "# This file is auto generated, do not edit.\n\nresources:\n\n"

	tmplFile := filepath.Join(p.cfg.confDir, "clusters.yaml")
	yamlText, err := os.ReadFile(tmplFile)
	if err != nil {
		return errors.AddStack(err)
	}
	yamlData := parser.parseYAML(string(yamlText), p.cfg)
	yamlData = parser.solveCommands(yamlData)

	outFile := filepath.Join(p.cfg.OutputPath(), "clusters.yaml")
	return p.writeYaml(outFile, yamlData, header)
}

func (p *ConfigGenerator) writeYaml(file string, data any, header string) error {
	var buf bytes.Buffer
	buf.WriteString(header)

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(data)
	if err != nil {
		return errors.AddStack(err)
	}

	err = easy.WriteFile(file, buf.Bytes(), 0644)
	if err != nil {
		return errors.AddStack(err)
	}
	return nil
}
