package main

import (
	"os"

	"context"

	"github.com/rancher/rancher/pkg/catalog/controller"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/client-go/tools/clientcmd"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "catalog-controller"
	app.Version = VERSION
	app.Author = "Rancher Labs, Inc."
	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug",
		},
		cli.StringFlag{
			Name:   "config",
			Usage:  "Kube config for accessing kubernetes cluster",
			EnvVar: "KUBECONFIG",
		},
		cli.StringFlag{
			Name:  "cache-root",
			Usage: "Cache root for catalog controller",
		},
		cli.IntFlag{
			Name:  "refresh-interval",
			Usage: "Refresh interval for catalog",
			Value: 60,
		},
	}

	app.Action = run
	app.Run(os.Args)
}

func run(ctx *cli.Context) error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", ctx.String("config"))
	if err != nil {
		return err
	}

	management, err := config.NewManagementContext(*kubeConfig)
	if err != nil {
		return err
	}

	controller.Run(context.Background(), ctx.String("cache-root"), ctx.Int("refresh-interval"), management)
	return management.StartAndWait()
}
