package cmd

import (
	"fmt"

	"github.com/rancher/rke/cluster"
	"github.com/urfave/cli"
)

func VersionCommand() cli.Command {
	versionFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  cluster.DefaultClusterConfig,
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
	localKubeConfig := cluster.GetLocalKubeConfig(ctx.String("config"))
	serverVersion, err := cluster.GetK8sVersion(localKubeConfig)
	if err != nil {
		return err
	}
	fmt.Printf("Server Version: %s\n", serverVersion)
	return nil
}
