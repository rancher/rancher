package main

import (
	"context"
	"os"

	controller "github.com/rancher/rancher/pkg/management/controller"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "cluster-manager-config",
			Usage: "Kube config for accessing cluster manager",
		},
		cli.StringFlag{
			Name:  "cluster-config",
			Usage: "Kube config for accessing cluster",
		},
		cli.StringFlag{
			Name:  "cluster-name",
			Usage: "name of the cluster",
		},
	}

	app.Action = func(c *cli.Context) error {
		return runControllers(
			c.String("cluster-manager-config"),
			c.String("cluster-config"),
			c.String("cluster-name"),
		)
	}

	app.ExitErrHandler = func(c *cli.Context, err error) {
		logrus.Fatal(err)
	}

	app.Run(os.Args)
}

func runControllers(clusterManagerCfg string, clusterCfg string, clusterName string) error {
	clusterManagementKubeConfig, err := clientcmd.BuildConfigFromFlags("", clusterManagerCfg)
	if err != nil {
		return err
	}

	clusterKubeConfig, err := clientcmd.BuildConfigFromFlags("", clusterCfg)
	if err != nil {
		return err
	}

	cluster, err := config.NewClusterContext(*clusterManagementKubeConfig, *clusterKubeConfig, clusterName)
	if err != nil {
		return err
	}

	ctx := context.Background()
	controller.Register(ctx, cluster)
	return cluster.StartAndWait(ctx)
}
