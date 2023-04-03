package envoy

import (
	"os"
	"os/exec"

	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/v2/zlog"
)

func Run(cfg *Configuration) (chan struct{}, error) {
	zlog.Infof("generating envoy config files")
	gen := NewConfigGenerator(cfg)
	err := gen.Generate()
	if err != nil {
		return nil, errors.WithMessage(err, "generate config files")
	}

	cmdArgs := []string{
		"-c", gen.BootstrapConfigFile(),
	}
	if cfg.LogLevel != "" {
		cmdArgs = append(cmdArgs, "-l", cfg.LogLevel)
	}

	command := exec.Command("envoy", cmdArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	zlog.Infof("starting envoy server: %v", command.String())
	err = command.Start()
	if err != nil {
		return nil, errors.WithMessage(err, "start envoy")
	}

	exit := make(chan struct{})
	go func() {
		command.Wait()
		close(exit)
	}()

	return exit, nil
}
