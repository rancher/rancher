package cmd

import (
	"context"
	"fmt"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"path/filepath"
)

var commonFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "ssh-agent-auth",
		Usage: "Use SSH Agent Auth defined by SSH_AUTH_SOCK",
	},
	cli.BoolFlag{
		Name:  "ignore-docker-version",
		Usage: "Disable Docker version check",
	},
}

func resolveClusterFile(ctx *cli.Context) (string, string, error) {
	clusterFile := ctx.String("config")
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", "", fmt.Errorf("Can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)
	return clusterFileBuff, clusterFile, nil
}

func setOptionsFromCLI(c *cli.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (*v3.RancherKubernetesEngineConfig, error) {
	// If true... override the file.. else let file value go through
	if c.Bool("ssh-agent-auth") {
		rkeConfig.SSHAgentAuth = c.Bool("ssh-agent-auth")
	}

	if c.Bool("ignore-docker-version") {
		rkeConfig.IgnoreDockerVersion = c.Bool("ignore-docker-version")
	}

	return rkeConfig, nil
}

func ClusterInit(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dialersOptions hosts.DialersOptions, flags cluster.ExternalFlags) error {
	log.Infof(ctx, "Initiating Kubernetes cluster")
	var fullState *cluster.FullState
	stateFilePath := cluster.GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir)
	rkeFullState, _ := cluster.ReadStateFile(ctx, stateFilePath)

	kubeCluster, err := cluster.InitClusterObject(ctx, rkeConfig, flags)
	if err != nil {
		return err
	}

	if err := kubeCluster.SetupDialers(ctx, dialersOptions); err != nil {
		return err
	}

	err = doUpgradeLegacyCluster(ctx, kubeCluster, rkeFullState)
	if err != nil {
		log.Warnf(ctx, "[state] can't fetch legacy cluster state from Kubernetes")
	}
	// check if certificate rotate or normal init
	if kubeCluster.RancherKubernetesEngineConfig.RotateCertificates != nil {
		fullState, err = rotateRKECertificates(ctx, kubeCluster, flags, rkeFullState)
	} else {
		fullState, err = cluster.RebuildState(ctx, &kubeCluster.RancherKubernetesEngineConfig, rkeFullState, flags)
	}
	if err != nil {
		return err
	}

	rkeState := cluster.FullState{
		DesiredState: fullState.DesiredState,
		CurrentState: fullState.CurrentState,
	}
	return rkeState.WriteStateFile(ctx, stateFilePath)
}
