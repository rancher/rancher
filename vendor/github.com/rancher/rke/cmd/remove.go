package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func RemoveCommand() cli.Command {
	removeFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  cluster.DefaultClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
		cli.BoolFlag{
			Name:  "force",
			Usage: "Force removal of the cluster",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "Deploy Kubernetes cluster locally",
		},
	}
	return cli.Command{
		Name:   "remove",
		Usage:  "Teardown the cluster and clean cluster nodes",
		Action: clusterRemoveFromCli,
		Flags:  removeFlags,
	}
}

func ClusterRemove(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dialerFactory hosts.DialerFactory,
	local bool, configDir string) error {

	logrus.Infof("Tearing down Kubernetes cluster")
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dialerFactory, nil)
	if err != nil {
		return err
	}

	err = kubeCluster.TunnelHosts(ctx, local)
	if err != nil {
		return err
	}

	logrus.Debugf("Starting Cluster removal")
	err = kubeCluster.ClusterRemove(ctx)
	if err != nil {
		return err
	}

	log.Infof(ctx, "Cluster removed successfully")
	return nil
}

func clusterRemoveFromCli(ctx *cli.Context) error {
	var local bool
	force := ctx.Bool("force")
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Are you sure you want to remove Kubernetes cluster [y/n]: ")
		input, err := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if err != nil {
			return err
		}
		if input != "y" && input != "Y" {
			return nil
		}
	}
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath
	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}
	if ctx.Bool("local") {
		rkeConfig.Nodes = []v3.RKEConfigNode{*cluster.GetLocalRKENodeConfig()}
		local = true
	}
	return ClusterRemove(context.Background(), rkeConfig, nil, local, "")
}
