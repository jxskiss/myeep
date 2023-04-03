package envoy

import (
	"path/filepath"

	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/v2/confr"
	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/zlog"
)

type Configuration struct {
	NodeCluster string `yaml:"nodeCluster" env:"ENVOY_NODE_CLUSTER" default:"infra.myeep.default"`
	NodeId      string `yaml:"nodeId" env:"ENVOY_NODE_ID"`
	AdminPort   int    `yaml:"adminPort" env:"ENVOY_ADMIN_PORT" default:"9000"`
	LogLevel    string `yaml:"logLevel" flag:"level" default:"info"`

	SSLCertServer struct {
		Enable     bool   `yaml:"enable"`
		HTTPAddr   string `yaml:"httpAddr"`
		SDSAddr    string `yaml:"sdsAddr"`
		CACert     string `yaml:"caCert"`
		ClientCert string `yaml:"clientCert"`
		ClientKey  string `yaml:"clientKey"`
	} `yaml:"sslCertServer"`

	confDir string
}

func ReadConfig(confDir string) (*Configuration, error) {
	confFile := filepath.Join(confDir, "envoy.yaml")
	cfg := &Configuration{}
	err := confr.New(&confr.Config{}).Load(cfg, confFile)
	if err != nil {
		return nil, errors.WithMessage(err, "failed read configuration")
	}
	cfg.confDir = confDir
	zlog.Infof("envoy configuration: %v", easy.JSON(cfg))
	return cfg, nil
}

func (cfg *Configuration) OutputPath() string {
	return filepath.Join(cfg.confDir, "generated")
}
