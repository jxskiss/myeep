package main

import (
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/jxskiss/gopkg/v2/fastrand"
	"github.com/jxskiss/gopkg/v2/zlog"
	"github.com/jxskiss/mcli"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type xdsProxyArgs struct {
	UnixSocket string   `cli:"-U, --unix-socket, Unix-socket to listen on" default:"/tmp/myeep-xdsproxy.sock"`
	XdsServers []string `cli:"#R, -x, --xds-server, XDS server to proxy connection to"`
}

func cmdXdsProxy(ctx *mcli.Context) {
	var args xdsProxyArgs
	ctx.Parse(&args)
	proxy := newXdsProxy(args)
	proxy.Run()
}

func runXdsProxy(unixSocket string, xdsAddrs []string) {

	// TODO: logging

	binName, _ := os.Executable()
	args := []string{"xds", "proxy", "-U", unixSocket}
	for _, addr := range xdsAddrs {
		args = append(args, "-x", addr)
	}
	proxyCmd := exec.Command(binName, args...)
	err := proxyCmd.Start()
	if err != nil {
		zlog.Fatalf("failed to start xds proxy: %v", err)
	}
	zlog.Infof("started xds proxy")
}

type xdsProxy struct {
	unixSocket string

	xdsAddrs []string
	xdsIdx   atomic.Uint32

	closed chan struct{}
	mtime  atomic.Value // time.Time
	wg     sync.WaitGroup

	log *zap.SugaredLogger
}

func newXdsProxy(args xdsProxyArgs) *xdsProxy {
	xdsAddrs := args.XdsServers
	fastrand.Shuffle(len(xdsAddrs), func(i, j int) {
		xdsAddrs[i], xdsAddrs[j] = xdsAddrs[j], xdsAddrs[i]
	})
	proxy := &xdsProxy{
		unixSocket: args.UnixSocket,
		xdsAddrs:   xdsAddrs,
		closed:     make(chan struct{}),
		log:        zlog.Named("xdsProxy").Sugar(),
	}
	return proxy
}

func (p *xdsProxy) Run() {
	unixSocket := p.unixSocket
	if err := os.RemoveAll(unixSocket); err != nil {
		p.log.Fatalf("cannot remove existing socket file %q: err= %v", unixSocket, err)
	}

	listener, err := net.Listen("unix", unixSocket)
	if err != nil {
		p.log.Fatalf("cannot listen unix socket: err= %v", err)
	}
	defer p.wg.Wait()
	defer listener.Close()

	p.touch()
	go p.keepAlive(listener)

	p.log.Infof("proxy started, waiting connections")
	for {
		envoyConn, err := listener.Accept()
		p.touch()
		if err != nil {
			select {
			case <-p.closed:
				return
			default:
				// pass
			}
			if opErr, ok := err.(*net.OpError); ok && opErr.Temporary() {
				continue
			}
			p.log.Fatalf("failed accepting connection: %v", err)
		}
		p.log.Infof("accepted connection")

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer p.touch()
			defer envoyConn.Close()
			xdsAddr := p.getXdsAddress()
			xdsConn, err := net.Dial("tcp", xdsAddr)
			if err != nil {
				p.log.Errorf("cannot dial xds server: addr= %v, err= %v", xdsAddr, err)
				return
			}
			defer xdsConn.Close()
			p.log.Infof("connected to xds server: addr= %v", xdsAddr)
			closer := make(chan struct{}, 2)
			go proxyCopy(closer, xdsConn, envoyConn)
			go proxyCopy(closer, envoyConn, xdsConn)
			<-closer
			p.log.Infof("proxy connection complete")
		}()
	}
}

func proxyCopy(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)

	// Connection is closed, send signal to stop proxy.
	closer <- struct{}{}
}

func (p *xdsProxy) getXdsAddress() string {
	if len(p.xdsAddrs) == 1 {
		return p.xdsAddrs[0]
	}
	next := p.xdsIdx.Add(1) % uint32(len(p.xdsAddrs))
	return p.xdsAddrs[next]
}

func (p *xdsProxy) touch() {
	p.mtime.Store(time.Now())
}

func (p *xdsProxy) keepAlive(lis net.Listener) {
	interval := 5 * time.Minute
	for {
		time.Sleep(interval)
		p.wg.Wait()
		mtime := p.mtime.Load().(time.Time)
		if time.Since(mtime) > interval {
			p.log.Infof("proxy is inactive, closing it")
			close(p.closed)
			lis.Close()
			return
		}
	}
}
