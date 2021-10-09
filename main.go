package main

import (
	"os"

	"github.com/jxskiss/gopkg/exp/zlog"
	"github.com/urfave/cli/v2"

	"github.com/jxskiss/myxdsdemo/envoy"
	"github.com/jxskiss/myxdsdemo/myxds"
	"github.com/jxskiss/myxdsdemo/myxds/provider"
)

func main() {
	zlog.SetupGlobals(&zlog.Config{Development: true}, true)
	defer zlog.Sync()

	app := cli.NewApp()
	app.Name = "myxdsdemo"
	app.HelpName = "myxdsdemo"
	app.Usage = "My Envoy xds demo application"

	app.Commands = []*cli.Command{
		envoyCommand(),
		xdsCommand(),
	}

	err := app.Run(os.Args)
	if err != nil {
		zlog.Fatal(err.Error())
	}
}

func envoyCommand() *cli.Command {
	return &cli.Command{
		Name:   "envoy",
		Usage:  "Run a envoy server",
		Action: envoyAction,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "log-level", Aliases: []string{"l"}, Usage: "set envoy log-level"},
		},
	}
}

// TODO: envoy hot-restart
//       https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/hot_restart
//       https://www.envoyproxy.io/docs/envoy/latest/operations/hot_restarter

func envoyAction(ctx *cli.Context) error {
	cfg, err := envoy.ReadConfig("./conf/envoy.yaml")
	if err != nil {
		return err
	}
	if level := ctx.String("log-level"); level != "" {
		cfg.LogLevel = level
	}
	exit, err := envoy.Run(cfg)
	if err != nil {
		return err
	}
	<-exit
	return nil
}

func xdsCommand() *cli.Command {
	return &cli.Command{
		Name:   "xds",
		Usage:  "Run the xds server",
		Action: xdsAction,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config-dir", Value: "./example/xdsconfig", Usage: "set xds config directory"},
			&cli.StringFlag{Name: "listen", Value: "127.0.0.1:8941", Usage: "set xds listen address"},
		},
	}
}

func xdsAction(ctx *cli.Context) error {
	prov := provider.NewFileProvider(ctx.String("config-dir"))
	exit, err := myxds.Run(prov, ctx.String("listen"))
	if err != nil {
		return err
	}
	<-exit
	return nil
}
