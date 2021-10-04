package envoy

import (
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/easy"
	"github.com/jxskiss/gopkg/exp/confr"
	"github.com/jxskiss/gopkg/exp/zlog"
)

type Configuration struct {
	Cluster    string   `yaml:"cluster" env:"ENVOY_CLUSTER" default:"infra.l7lb.myxds_demo_envoy"`
	NodeId     string   `yaml:"nodeId" env:"ENVOY_NODE_ID" default:"a_dummy_demo_envoy_node_id"`
	LogLevel   string   `yaml:"logLevel" flag:"level"`
	XdsServers []string `yaml:"xdsServers"`
}

func ReadConfig(path string) (*Configuration, error) {
	cfg := &Configuration{}
	err := confr.New(&confr.Config{}).Load(cfg, path)
	if err != nil {
		return nil, errors.WithMessage(err, "failed read configuration")
	}
	zlog.Infof("envoy configuration: %v", easy.Pretty(cfg))
	return cfg, nil
}

func Run(cfg *Configuration) (chan struct{}, error) {
	zlog.Infof("building envoy bootstrap config")
	bootstrapData := &BootstrapData{
		Cluster:    cfg.Cluster,
		NodeId:     cfg.NodeId,
		XdsServers: parseXdsServerAddresses(cfg.XdsServers),
	}
	bootstrapConfig, err := buildBootstrapConfig(bootstrapData)
	if err != nil {
		return nil, errors.WithMessage(err, "failed build bootstrap config")
	}
	bootstrapConfigFile := "./conf/generated/envoy-bootstrap.yaml"
	err = ioutil.WriteFile(bootstrapConfigFile, bootstrapConfig, 0666)
	if err != nil {
		return nil, errors.WithMessage(err, "failed write bootstrap config")
	}

	cmdArgs := []string{
		"-c", bootstrapConfigFile,
	}
	if cfg.LogLevel != "" {
		cmdArgs = append(cmdArgs, "-l", cfg.LogLevel)
	}

	command := exec.Command("envoy", cmdArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	zlog.Infof("starting envoy: %v", command.String())
	err = command.Start()
	if err != nil {
		return nil, errors.WithMessage(err, "failed start envoy")
	}

	exit := make(chan struct{})
	go func() {
		command.Wait()
		close(exit)
	}()

	return exit, nil
}
