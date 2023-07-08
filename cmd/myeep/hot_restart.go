package main

import (
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/zlog"
	"go.uber.org/zap"
)

/*
Hot restarter.

References:
- https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/hot_restart
- https://www.envoyproxy.io/docs/envoy/latest/operations/hot_restarter
- https://github.com/envoyproxy/envoy/blob/main/restarter/hot-restarter.py

*/

// TERM_WAIT_SECONDS is the number of seconds to wait for children to gracefully
// exit after propagating SIGTERM before force killing children.
//
// NOTE: If using a shutdown mechanism such as runit's `force-stop` which sends
// a KILL after a specified timeout period, it's important to ensure that this
// constant is smaller than the KILL timeout.
const TERM_WAIT_SECONDS = 30

type hrArgs struct {
	ConfDir string
	BinPath string
}

type hotRestarter struct {
	epoch   int
	pidList []int

	args   *hrArgs
	logger *zap.SugaredLogger

	sigMu          sync.Mutex
	sigCh          chan os.Signal
	disableSigchld bool
}

func newHotRestarter(args *hrArgs) *hotRestarter {
	hr := &hotRestarter{
		args:   args,
		logger: zlog.Named("hotRestarter").Sugar(),
	}
	return hr
}

// Run starts the hot restarter.
// This script is designed so that a process watcher like runit or monit can watch
// this process and take corrective action if it ever goes away.
func (p *hotRestarter) Run() {
	p.logger.Infof("starting hot-restarter with target: %v", p.args.BinPath)

	p.sigCh = make(chan os.Signal)
	signal.Notify(p.sigCh,
		syscall.SIGTERM, syscall.SIGINT,
		syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGCHLD,
	)

	// Start the first child process.
	p.forkAndExec()

	processSignal := func(sig os.Signal) {
		p.sigMu.Lock()
		defer p.sigMu.Unlock()
		switch sig.(syscall.Signal) {
		case syscall.SIGTERM:
			p.sigtermHandler()
		case syscall.SIGINT:
			p.sigintHandler()
		case syscall.SIGHUP:
			p.sighupHandler()
		case syscall.SIGUSR1:
			p.sigusr1Handler()
		case syscall.SIGCHLD:
			if !p.disableSigchld {
				p.sigchldHandler()
			}
		}
	}

	// Block on signals since everything else happens via signals.
	p.logger.Infof("waiting signals")
	for sig := range p.sigCh {
		processSignal(sig)
	}
}

func (p *hotRestarter) removePid(pid int) {
	p.pidList = easy.Filter(
		func(_ int, elem int) bool { return elem != pid },
		p.pidList)
}

// Iterate through all known child processes, send a TERM signal to each of
// them, and then wait up to TERM_WAIT_SECONDS for them to exit gracefully,
// exiting early if all children go away. If one or more children have not
// exited after TERM_WAIT_SECONDS, they will be forcibly killed.
func (p *hotRestarter) termAllChildren() {

	// First disable the SIGCHLD handler so that we don't get called again.
	p.disableSigchld = true

	for _, pid := range p.pidList {
		p.logger.Infof("sending TERM to PID=%d", pid)
		err := syscall.Kill(pid, syscall.SIGTERM)
		if err != nil {
			p.logger.Errorf("error sending TERM to PID=%v, countinuing, err= %v", pid, err)
		}
	}

	allExited := false

	// Wait for TERM_WAIT_SECONDS seconds for children to exit cleanly.
	retries := 0
	for !allExited && retries < TERM_WAIT_SECONDS {
		pidListCopy := easy.Copy(p.pidList)
		for _, pid := range pidListCopy {
			var (
				status  syscall.WaitStatus
				rusage  syscall.Rusage
				retPid  int
				waitErr error
			)
			retPid, waitErr = syscall.Wait4(pid, &status, syscall.WNOHANG, &rusage)
			if waitErr != nil {
				p.logger.Errorf("error wait child PID=%d to exit, continuing, err= %v", pid, waitErr)
			}
			if retPid == 0 && status == 0 {
				// The child is still running.
				continue
			}

			p.removePid(pid)
		}

		if len(p.pidList) == 0 {
			allExited = true
		} else {
			retries += 1
			time.Sleep(time.Second)
		}
	}

	if allExited {
		p.logger.Infof("all children exited cleanly")
	} else {
		for _, pid := range p.pidList {
			p.logger.Infof("child PID=%d did not exit cleanly, killing", pid)
		}
		p.forceKillAllChildren()

		// Return error status because children did not exit cleanly.
		os.Exit(1)
	}
}

// Iterate through all known child processes and force kill them.
// Typically, termAllChildren() should be attempted first to give child processes
// an opportunity to clean up state before exiting.
func (p *hotRestarter) forceKillAllChildren() {
	for _, pid := range p.pidList {
		p.logger.Infof("force killing PID=%d", pid)
		err := syscall.Kill(pid, syscall.SIGKILL)
		if err != nil {
			p.logger.Errorf("error force killing PID=%d, continuing, err= %v", pid, err)
		}
	}
	p.pidList = nil
}

// Attempt to gracefully shutdown all child Envoy processes and then exit.
// See termAllChildren() for further discussion.
func (p *hotRestarter) shutdown() {
	p.termAllChildren()
	os.Exit(0)
}

// Handler for SIGTERM.
func (p *hotRestarter) sigtermHandler() {
	p.logger.Warnf("got SIGTERM")
	p.shutdown()
}

// Handler for SIGINT (ctrl-c). The same as the SIGTERM handler.
func (p *hotRestarter) sigintHandler() {
	p.logger.Warnf("got SIGINT")
	p.shutdown()
}

// Handler for SIGHUP.
// This signal is used to cause the restarter to fork and exec a new child.
func (p *hotRestarter) sighupHandler() {
	p.logger.Infof("got SIGHUP")
	p.forkAndExec()
}

// Handler for SIGUSR1. Propagate SIGUSR1 to all the child processes.
func (p *hotRestarter) sigusr1Handler() {
	for _, pid := range p.pidList {
		p.logger.Infof("sending SIGUSR1 to PID=%d", pid)
		err := syscall.Kill(pid, syscall.SIGUSR1)
		if err != nil {
			p.logger.Errorf("error sending SIGUSR1 to PID=%d, countinuing, err= %v", pid, err)
		}
	}
}

// Handler for SIGCHLD.
// Iterates through all of our known child processes and figures out whether
// the signal/exit was expected or not.
// Python doesn't have any of the native signal handlers ability to get
// the child process info directly from the signal handler, so we need to
// iterate through all child processes and see what happened.
func (p *hotRestarter) sigchldHandler() {
	p.logger.Infof("got SIGCHLD")

	killAllAndExit := false
	pidListCopy := easy.Copy(p.pidList)
	for _, pid := range pidListCopy {
		var (
			status  syscall.WaitStatus
			rusage  syscall.Rusage
			retPid  int
			waitErr error
		)
		retPid, waitErr = syscall.Wait4(pid, &status, syscall.WNOHANG, &rusage)
		if waitErr != nil {
			p.logger.Errorf("error wait child PID=%d to exit, continuing, err= %v", pid, waitErr)
		}
		if retPid == 0 && status == 0 {
			// The child is still running.
			continue
		}

		p.removePid(pid)

		// Now we see how the child exited.
		if status.Exited() {
			exitCode := status.ExitStatus()
			if exitCode == 0 {
				// Normal exit. We assume this was on purpose.
				p.logger.Infof("PID=%d exited with code=0", retPid)
			} else {
				// Something bad happened. We need to tear everything down
				// so that whoever started the restarter can know about
				// this situation and restart the whole thing.
				p.logger.Warnf("PID=%d exited with code=%d", retPid, exitCode)
				killAllAndExit = true
			}
		} else if status.Signaled() {
			p.logger.Warnf("PID=%d was killed with signal=%v", retPid, status.Signal())
			killAllAndExit = true
		} else {
			killAllAndExit = true
		}
	}

	if killAllAndExit {
		p.logger.Warnf("Due to abnormal exit, force killing all child processes and exiting")

		// First disable the SIGCHLD handler so that we don't get called again.
		p.disableSigchld = true

		p.forceKillAllChildren()
	}

	// Our last child died, so we have no purpose. Exit.
	if len(p.pidList) == 0 {
		p.logger.Warnf("exiting due to lack of child processes")
		if killAllAndExit {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// This routine forks and execs a new child process and keeps track of its PID.
// Before we fork, set the current restart epoch in an env variable that
// processes can read if they care.
func (p *hotRestarter) forkAndExec() {
	restartEpoch := p.epoch
	p.logger.Infof("forking and execing new child process at epoch %d", restartEpoch)
	p.epoch += 1

	copyEnv := os.Environ()
	copyEnv = setEnv(copyEnv, "RESTART_EPOCH", strconv.Itoa(restartEpoch))
	copyEnv = setEnv(copyEnv, "START_BY_HOT_RESTARTER", "1")

	wd, _ := os.Getwd()
	childArgs := []string{
		p.args.BinPath,
		"envoy", "run", "-c", p.args.ConfDir,
	}
	childPid, err := syscall.ForkExec(
		childArgs[0],
		childArgs,
		&syscall.ProcAttr{
			Dir:   wd,
			Env:   copyEnv,
			Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
			Sys:   nil,
		})
	if err != nil {
		p.logger.Errorf("error fork and exec child process at epoch %d, force killing all child processes and exiting, err= %v", restartEpoch, err)

		// First disable the SIGCHLD handler so that we don't get called again.
		p.disableSigchld = true

		p.forceKillAllChildren()
		os.Exit(1)
	}

	p.logger.Infof("forked new child process with PID=%d", childPid)
	p.pidList = append(p.pidList, childPid)
}

func setEnv(envList []string, name, value string) []string {
	prefix := name + "="
	found := false
	for i, s := range envList {
		if strings.HasPrefix(s, prefix) {
			found = true
			envList[i] = prefix + value
			break
		}
	}
	if !found {
		envList = append(envList, prefix+value)
	}
	return envList
}
