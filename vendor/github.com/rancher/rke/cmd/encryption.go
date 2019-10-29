package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func EncryptionCommand() cli.Command {
	encryptFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  pki.ClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
	}
	encryptFlags = append(encryptFlags, commonFlags...)
	return cli.Command{
		Name:  "encrypt",
		Usage: "Manage cluster encryption provider keys",
		Subcommands: cli.Commands{
			cli.Command{
				Name:   "rotate-key",
				Usage:  "Rotate cluster encryption provider key",
				Action: rotateEncryptionKeyFromCli,
				Flags:  encryptFlags,
			},
		},
	}
}

func rotateEncryptionKeyFromCli(ctx *cli.Context) error {
	logrus.Infof("Running RKE version: %v", ctx.App.Version)
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return err
	}

	// setting up the flags
	flags := cluster.GetExternalFlags(false, false, false, "", filePath)

	return RotateEncryptionKey(context.Background(), rkeConfig, hosts.DialersOptions{}, flags)
}

func RotateEncryptionKey(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig,
	dialersOptions hosts.DialersOptions, flags cluster.ExternalFlags) error {
	log.Infof(ctx, "Rotating cluster secrets encryption key..")
	stateFilePath := cluster.GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir)
	rkeFullState, _ := cluster.ReadStateFile(ctx, stateFilePath)
	// We generate the first encryption config in ClusterInit, to store it ASAP. It's written
	// to the DesiredState
	stateEncryptionConfig := rkeFullState.DesiredState.EncryptionConfig

	// if CurrentState has EncryptionConfig, it means this is NOT the first time we enable encryption, we should use the _latest_ applied value from the current cluster
	if rkeFullState.CurrentState.EncryptionConfig != "" {
		stateEncryptionConfig = rkeFullState.CurrentState.EncryptionConfig
	}

	kubeCluster, err := cluster.InitClusterObject(ctx, rkeConfig, flags, stateEncryptionConfig)
	if err != nil {
		return err
	}
	if kubeCluster.IsEncryptionCustomConfig() {
		return fmt.Errorf("can't rotate encryption keys: Key Rotation is not supported with custom configuration")
	}
	if !kubeCluster.IsEncryptionEnabled() {
		return fmt.Errorf("can't rotate encryption keys: Encryption Configuration is disabled")
	}
	kubeCluster.Certificates = rkeFullState.DesiredState.CertificatesBundle
	if err := kubeCluster.SetupDialers(ctx, dialersOptions); err != nil {
		return err
	}
	if err := kubeCluster.TunnelHosts(ctx, flags); err != nil {
		return err
	}

	err = kubeCluster.RotateEncryptionKey(ctx, rkeFullState)
	if err != nil {
		return err
	}
	// make sure we have the latest state
	rkeFullState, _ = cluster.ReadStateFile(ctx, stateFilePath)
	log.Infof(ctx, "Reconciling cluster state")
	if err := kubeCluster.ReconcileDesiredStateEncryptionConfig(ctx, rkeFullState); err != nil {
		return err
	}
	log.Infof(ctx, "Cluster secrets encryption key rotated successfully")
	return nil
}
