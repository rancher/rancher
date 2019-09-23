package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	"github.com/urfave/cli"
)

func VersionCommand() cli.Command {
	versionFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  pki.ClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
	}
	return cli.Command{
		Name:   "version",
		Usage:  "Show cluster Kubernetes version",
		Action: getClusterVersion,
		Flags:  versionFlags,
	}
}

func getClusterVersion(ctx *cli.Context) error {
	logrus.Infof("Running RKE version: %v", ctx.App.Version)
	localKubeConfig := pki.GetLocalKubeConfig(ctx.String("config"), "")
	// not going to use a k8s dialer here.. this is a CLI command
	serverVersion, err := cluster.GetK8sVersion(localKubeConfig, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Server Version: %s\n", serverVersion)
	return nil
}
