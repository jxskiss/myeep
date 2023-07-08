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

	header := "# This file is auto generated, do not edit.\n\n"

	yamlData, err := parser.parseYAML(bootstrapTpl, p.cfg)
	if err != nil {
		return errors.WithMessage(err, "parse bootstrap yaml")
	}
	yamlData, err = parser.solveCommands("bootstrap", yamlData)
	if err != nil {
		return errors.WithMessage(err, "solve commands in bootstrap yaml")
	}

	outFile := p.BootstrapConfigFile()
	return p.writeYaml(outFile, yamlData, header)
}

func (p *ConfigGenerator) genListenersConfig() error {
	parser := &YAMLParser{
		cfg: p.cfg,
	}

	header := "# This file is auto generated, do not edit.\n\nresources:\n\n"

	tmplFile := filepath.Join(p.cfg.confDir, "listeners.yaml")
	yamlText, err := os.ReadFile(tmplFile)
	if err != nil {
		return errors.WithMessage(err, "read listeners.yaml")
	}
	yamlData, err := parser.parseYAML(string(yamlText), p.cfg)
	if err != nil {
		return errors.WithMessage(err, "parse listeners.yaml")
	}
	yamlData, err = parser.solveCommands("listeners", yamlData)
	if err != nil {
		return errors.WithMessage(err, "solve commands in listeners.yaml")
	}

	outFile := filepath.Join(p.cfg.OutputPath(), "listeners.yaml")
	return p.writeYaml(outFile, yamlData, header)
}

var simpleSSLClusterTpl = []byte(`
# simplessl
- "!@@ simple_cluster":
    name: "{{ .SimpleSSL.ClusterName }}"
    endpoints:
      - "{{ .SimpleSSL.HTTPAddr }}"
`)

func (p *ConfigGenerator) genClustersConfig() error {
	parser := &YAMLParser{
		cfg: p.cfg,
	}

	header := "# This file is auto generated, do not edit.\n\nresources:\n\n"

	tmplFile := filepath.Join(p.cfg.confDir, "clusters.yaml")
	yamlText, err := os.ReadFile(tmplFile)
	if err != nil {
		return errors.WithMessage(err, "read clusters.yaml")
	}

	if p.cfg.SimpleSSL.Enable {
		yamlText = append(simpleSSLClusterTpl, yamlText...)
	}

	yamlData, err := parser.parseYAML(string(yamlText), p.cfg)
	if err != nil {
		return errors.WithMessage(err, "parse clusters.yaml")
	}
	yamlData, err = parser.solveCommands("clusters", yamlData)
	if err != nil {
		return errors.WithMessage(err, "solve commands in clusters.yaml")
	}

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
		return errors.WithMessagef(err, "encoding yaml file %s", file)
	}

	err = easy.WriteFile(file, buf.Bytes(), 0644)
	if err != nil {
		return errors.WithMessagef(err, "writing yaml file %s", file)
	}
	return nil
}
