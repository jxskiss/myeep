package model

type DomainGroup struct {
	Name            string      `json:"name" yaml:"name"`
	Domains         []string    `json:"domains" yaml:"domains"`
	CertificateName string      `json:"certificate_name" yaml:"certificate_name"`
	Directives      []Directive `json:"directives" yaml:"directives"`
}

type ServiceGroup struct {
	DefaultDomainGroup string           `json:"default_domain_group" yaml:"default_domain_group"`
	Services           []*Service       `json:"services" yaml:"services"`
	StaticServices     []*StaticService `json:"static_services" yaml:"static_services"`
}

type Service struct {
	Name       string      `json:"name" yaml:"name"`
	Directives []Directive `json:"directives" yaml:"directives"`
	Locations  []*Location `json:"locations" yaml:"locations"`
}

type StaticService struct {
	Upstream   string      `json:"upstream" yaml:"upstream"`
	Directives []Directive `json:"directives" yaml:"directives"`
	Locations  []*Location `json:"locations" yaml:"locations"`
}

type Location struct {
	Path              string         `json:"path" yaml:"path"`
	RePath            string         `json:"re_path" yaml:"re_path"`
	Directives        []Directive    `json:"directives" yaml:"directives"`
	RoutingRules      []*RoutingRule `json:"routing_rules" yaml:"routing_rules"`
	ExtraDomainGroups []string       `json:"extra_domain_groups" yaml:"extra_domain_groups"`
}

type RoutingRule struct {
	Type        string   `json:"type" yaml:"type"`
	Arguments   []string `json:"arguments" yaml:"arguments"`
	DestService string   `json:"dest_service" yaml:"dest_service"`
}
