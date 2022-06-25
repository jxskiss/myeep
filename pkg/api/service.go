package api

type Service struct {
	Name       string      `json:"name" yaml:"name"`
	Cluster    string      `json:"cluster" yaml:"cluster"`
	Directives []Directive `json:"directives" yaml:"directives"`
	Routes     []*Routing  `json:"routes" yaml:"routes"`
}

type Routing struct {
	DomainGroups []DomainGroupNameAndPorts `json:"domain_groups" yaml:"domain_groups"`
	Directives   []Directive               `json:"directives" yaml:"directives"`
	Locations    []*Location               `json:"locations" yaml:"locations"`
}

type Location struct {

	// Precisely one of Path, RegexPath must be set.
	Path      string `json:"path" yaml:"path"`
	RegexPath string `json:"regex_path" yaml:"regex_path"`

	Directives []Directive  `json:"directives" yaml:"directives"`
	Splitting  []*Splitting `json:"splitting" yaml:"splitting"`
}

type Splitting struct {
	Type      string   `json:"type" yaml:"type"`
	Arguments []string `json:"arguments" yaml:"arguments"`

	Directives  []Directive `json:"directives" yaml:"directives"`
	DestCluster string      `json:"dest_cluster" yaml:"dest_cluster"`
}
