package api

type Endpoint struct {
	Addr     string            `json:"addr" yaml:"addr"`
	Cluster  string            `json:"cluster" yaml:"cluster"`
	Env      string            `json:"env" yaml:"env"`
	Weight   float64           `json:"weight" yaml:"weight"`
	Metadata map[string]string `json:"metadata" yaml:"metadata"`
}
