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
	LogLevel    string `yaml:"logLevel" env:"ENVOY_LOG_LEVEL" flag:"log-level" default:"info"`

	MaxOpenFilesNum      uint64 `yaml:"maxOpenFilesNum" env:"ENVOY_MAX_OPEN_FILES" default:"102400"`
	MaxInotifyWatchesNum uint64 `yaml:"maxInotifyWatchesNum" env:"ENVOY_MAX_INOTIFY_WATCHES" default:"524288"`

	PassThroughFlags []string `yaml:"passThroughFlags"`

	SimpleSSL struct {
		Enable      bool   `yaml:"enable"`
		ClusterName string `yaml:"clusterName"`
		HTTPAddr    string `yaml:"httpAddr"`
		SDSAddr     string `yaml:"sdsAddr"`
		CACert      string `yaml:"caCert"`
		ClientCert  string `yaml:"clientCert"`
		ClientKey   string `yaml:"clientKey"`
	} `yaml:"simpleSSL"`

	confDir string
}

func ReadConfig(confDir string) (*Configuration, error) {
	confFile := filepath.Join(confDir, "envoy.yaml")
	cfg := &Configuration{}
	err := confr.New(&confr.Config{}).Load(cfg, confFile)
	if err != nil {
		return nil, errors.WithMessage(err, "read envoy configuration")
	}
	cfg.confDir = confDir
	zlog.Infof("envoy configuration: %v", easy.JSON(cfg))
	return cfg, nil
}

func (cfg *Configuration) OutputPath() string {
	return filepath.Join(cfg.confDir, "generated")
}
