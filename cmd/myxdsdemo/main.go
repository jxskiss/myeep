package main

import (
	"os"

	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/zlog"
	"github.com/jxskiss/mcli"

	"github.com/jxskiss/myxdsdemo/pkg/envoy"
	"github.com/jxskiss/myxdsdemo/pkg/myxds"
	"github.com/jxskiss/myxdsdemo/pkg/provider"
)

func main() {
	zlog.SetDevelopment()
	defer zlog.Sync()

	app := mcli.NewApp()
	app.Add("envoy", runEnvoy, "Run envoy server")
	app.Add("envoy dump", dumpEnvoyConfig, "Dump envoy configuration")
	app.Add("envoy tools", runEnvoyTools, "Run envoy config tools (not implemented)")
	app.Add("xds", runXdsServer, "Run xDS server")
	app.Add("xds proxy", cmdXdsProxy, "Run local xDS proxy")
	app.Run()
}

// TODO: envoy hot-restart
//       https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/hot_restart
//       https://www.envoyproxy.io/docs/envoy/latest/operations/hot_restarter

func runEnvoy(ctx *mcli.Context) {
	var args struct {
		LogLevel string `cli:"-l, --log-level, set envoy log-level"`
	}
	ctx.Parse(&args)

	cfg, err := envoy.ReadConfig("./conf/envoy.yaml")
	if err != nil {
		zlog.Fatalf("failed read config: %v", err)
	}
	if args.LogLevel != "" {
		cfg.LogLevel = args.LogLevel
	}

	// Start xds proxy in background.
	runXdsProxy(cfg.XDS.ProxySocket, cfg.XDS.Servers)

	exit, err := envoy.Run(cfg)
	if err != nil {
		zlog.Fatalf("failed run envoy: %v", err)
	}
	<-exit
}

func dumpEnvoyConfig(ctx *mcli.Context) {
	var args struct {
		IncludeEDS bool `cli:"--eds, Dump endpoints discovered from EDS"`
	}
	ctx.Parse(&args)

	url := "http://127.0.0.1:9000/config_dump"
	params := map[string]interface{}{}
	if args.IncludeEDS {
		params["include_eds"] = true
	}
	_, resp, _, err := easy.DoRequest(&easy.Request{
		URL:            url,
		Params:         params,
		RaiseForStatus: true,
	})
	if err != nil {
		zlog.Fatalf("failed query config_dump")
	}
	outFile := "./conf/generated/envoy-config-dump.json"
	err = os.WriteFile(outFile, resp, 0644)
	if err != nil {
		zlog.Fatalf("failed write dumped config: %v", err)
	}
}

func runEnvoyTools(ctx *mcli.Context) {
	// TODO
	// See https://github.com/vorishirne/envoyconf-tools?ref=golangexample.com
	panic("not implemented")
}

func runXdsServer(ctx *mcli.Context) {
	var args struct {
		ConfigDir string `cli:"--config-dir, set xds config directory" default:"./example/xdsconfig"`
		Listen    string `cli:"--listen, set xds listen address" default:"127.0.0.1:8941"`
	}
	ctx.Parse(&args)

	prov := provider.NewFileProvider(args.ConfigDir)
	exit, err := myxds.Run(prov, args.Listen)
	if err != nil {
		zlog.Fatalf("failed run xds sever: %v", err)
	}
	<-exit
}
