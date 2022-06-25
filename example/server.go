package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/zlog"
	"github.com/jxskiss/mcli"
)

func main() {
	zlog.SetDevelopment()
	defer zlog.Sync()

	var args struct {
		Ports string `cli:"#R, -p, ports to listen, split by comma"`
	}
	mcli.Parse(&args)
	portList := easy.ParseInts[int64](strings.Split(args.Ports, ","), 10)
	if len(portList) == 0 {
		zlog.Fatal("invalid argument -p")
	}

	for _, port := range portList {
		go func(port int) {
			addr := fmt.Sprintf("0.0.0.0:%d", port)
			lis, err := net.Listen("tcp", addr)
			if err != nil {
				zlog.Fatalf("can't listen to port %v: %v", port, err)
			}
			zlog.Infof("listening on %v", addr)
			_ = http.Serve(lis, &handler{port: port})
		}(int(port))
	}

	exit := make(chan struct{})
	<-exit
}

type handler struct {
	port int
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("(%d) %v %v %v", h.port, r.Method, r.Host, r.RequestURI)
	zlog.Infof("serving: %v", msg)

	w.WriteHeader(200)
	w.Write([]byte(msg))
}
