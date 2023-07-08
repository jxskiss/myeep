package envoy

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/v2/zlog"
	"github.com/lorenzosaino/go-sysctl"
)

func Run(cfg *Configuration, restartEpoch int) (chan struct{}, error) {
	zlog.Infof("generating envoy config files")
	gen := NewConfigGenerator(cfg)
	err := gen.Generate()
	if err != nil {
		return nil, errors.WithMessage(err, "generate config files")
	}

	setSystemParams(cfg)

	cmdArgs := []string{
		"-c", gen.BootstrapConfigFile(),
	}

	// restart-epoch specified by command line flag.
	if restartEpoch > 0 {
		cmdArgs = append(cmdArgs, "--restart-epoch", strconv.Itoa(restartEpoch))
	} else
	// hot-restarter
	if os.Getenv("START_BY_HOT_RESTARTER") == "1" {
		cmdArgs = append(cmdArgs, "--restart-epoch", os.Getenv("RESTART_EPOCH"))
	}

	cmdArgs = append(cmdArgs, cfg.PassThroughFlags...)
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

func setSystemParams(cfg *Configuration) {
	err := sysctl.Set("fs.inotify.max_user_watches", fmt.Sprint(cfg.MaxInotifyWatchesNum))
	if err != nil {
		zlog.Warnf("failed set 'fs.inotify.max_user_watches=%d', err= %v", cfg.MaxInotifyWatchesNum, err)
	} else {
		zlog.Infof("success set 'fs.inotify.max_user_watches=%d'", cfg.MaxInotifyWatchesNum)
	}

	rLimit := syscall.Rlimit{
		Cur: cfg.MaxOpenFilesNum,
		Max: cfg.MaxOpenFilesNum,
	}
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		zlog.Warnf("failed set 'ulimit -n %d', err= %v", cfg.MaxOpenFilesNum, err)
	} else {
		zlog.Infof("success set 'ulimit -n %d'", cfg.MaxOpenFilesNum)
	}
}
