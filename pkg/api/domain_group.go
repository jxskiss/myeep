package api

type DomainGroup struct {
	Name       string      `json:"name" yaml:"name"`
	Domains    []string    `json:"domains" yaml:"domains"`
	CertName   string      `json:"cert_name" yaml:"cert_name"`
	Directives []Directive `json:"directives" yaml:"directives"`
}

type DomainGroupNameAndPorts struct {
	Name  string `json:"name" yaml:"name"`
	Ports []uint `json:"ports" yaml:"ports"`
}
