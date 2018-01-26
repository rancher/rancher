package main

import (
	"context"
	"os"

	"github.com/rancher/rancher/pkg/management/controller"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Kube config for accessing kubernetes cluster",
			EnvVar: "KUBECONFIG",
		},
	}

	app.Action = func(c *cli.Context) error {
		return run(c.String("config"))
	}

	app.ExitErrHandler = func(c *cli.Context, err error) {
		logrus.Fatal(err)
	}

	app.Run(os.Args)
}

func run(kubeConfigFile string) error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return err
	}

	management, err := config.NewManagementContext(*kubeConfig)
	if err != nil {
		return err
	}

	controller.Register(context.Background(), management)

	return management.StartAndWait()
}
