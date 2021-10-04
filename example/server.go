package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"

	"github.com/jxskiss/gopkg/easy"
	"github.com/jxskiss/gopkg/exp/zlog"
)

var argPorts = flag.String("p", "", "ports to listen")

func main() {
	flag.Parse()
	zlog.ReplaceGlobals(zlog.MustNewLogger(&zlog.Config{Development: true}))

	portList, _ := easy.ParseInt64s(*argPorts, ",", true)
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
