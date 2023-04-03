package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/easy/ezhttp"
	"github.com/jxskiss/gopkg/v2/zlog"
	"github.com/jxskiss/mcli"
	"gopkg.in/yaml.v3"

	"github.com/jxskiss/myeep/pkg/envoy"
)

func main() {
	zlog.SetDevelopment()
	defer zlog.Sync()

	app := mcli.NewApp()
	app.Add("envoy run", runEnvoyServer, "Run envoy server")
	app.Add("envoy generate-config", generateEnvoyConfig, "Generate envoy config files")
	app.Add("envoy dump", dumpEnvoyConfig, "Dump envoy configuration from admin interface")
	app.Run()
}

// TODO: envoy hot-restart
//       https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/hot_restart
//       https://www.envoyproxy.io/docs/envoy/latest/operations/hot_restarter

func runEnvoyServer(ctx *mcli.Context) {
	var args struct {
		ConfDir  string `cli:"-c, --conf-dir, configuration directory" default:"./conf"`
		LogLevel string `cli:"-l, --log-level, set envoy log-level"`
	}
	ctx.Parse(&args)
	cfg, err := envoy.ReadConfig(args.ConfDir)
	if err != nil {
		zlog.Fatalf("failed read config: %v", err)
	}
	if args.LogLevel != "" {
		cfg.LogLevel = args.LogLevel
	}

	exit, err := envoy.Run(cfg)
	if err != nil {
		zlog.Fatalf("failed run envoy: %v", err)
	}
	<-exit
}

func generateEnvoyConfig(ctx *mcli.Context) {
	var args struct{
		ConfDir  string `cli:"-c, --conf-dir, configuration directory" default:"./conf"`

		// TODO: backup
	}
	ctx.Parse(&args)
	cfg, err := envoy.ReadConfig(args.ConfDir)
	if err != nil {
		zlog.Fatalf("failed read config: %v", err)
	}
	gen := envoy.NewConfigGenerator(cfg)
	err = gen.Generate()
	if err != nil {
		zlog.Fatalf("failed generate config files: %v", err)
	}
	zlog.Infof("success")
}

func dumpEnvoyConfig(ctx *mcli.Context) {
	var args struct {
		ConfDir      string `cli:"-c, --conf-dir, configuration directory" default:"./conf"`
		IncludeEDS   bool   `cli:"    --include-eds, dump endpoints discovered from EDS"`
		AddTimestamp bool   `cli:"-t, --add-timestamp, add timestamp to the output file's name"`
		Output       string `cli:"-o, --output, output file to dump configuration, use .yaml to output YAML instead of JSON" default:"./config_dump.json"`
	}
	ctx.Parse(&args)

	outputYAML := false
	if strings.HasSuffix(args.Output, ".yaml") || strings.HasSuffix(args.Output, ".yml") {
		outputYAML = true
	}

	getFilename := func() string {
		dir, filename := filepath.Split(args.Output)
		if args.AddTimestamp {
			filename += time.Now().Format("20060102150405_") + filename
		}
		return filepath.Join(dir, filename)
	}

	cfg, err := envoy.ReadConfig(args.ConfDir)
	if err != nil {
		zlog.Fatalf("failed read config: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%v/config_dump", cfg.AdminPort)
	params := map[string]any{}
	if args.IncludeEDS {
		params["include_eds"] = true
	}
	_, jsonResp, _, err := ezhttp.Do(&ezhttp.Request{
		URL:            url,
		Params:         params,
		RaiseForStatus: true,
	})
	if err != nil {
		zlog.Fatalf("failed query config_dump: %v", err)
	}

	fileData := jsonResp
	if outputYAML {
		var configDump map[string]any
		err = json.Unmarshal(jsonResp, &configDump)
		if err != nil {
			zlog.Fatalf("failed unmarshal config_dump response: %v", err)
		}
		fileData, err = yaml.Marshal(configDump)
		if err != nil {
			zlog.Fatalf("failed marshal config_dump data to YAML: %v", err)
		}
	}

	outFile := getFilename()
	err = easy.WriteFile(outFile, fileData, 0644)
	if err != nil {
		zlog.Fatalf("failed write dumped config: %v", err)
	}
}

//nolint:unused
func runEnvoyConfTools(ctx *mcli.Context) {
	// See https://github.com/vorishirne/envoyconf-tools
	panic("not implemented")
}
