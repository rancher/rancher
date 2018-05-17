package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
)

func EtcdCommand() cli.Command {
	backupRestoreFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "Specify Backup name",
		},
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  pki.ClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
	}

	backupRestoreFlags = append(backupRestoreFlags, commonFlags...)

	return cli.Command{
		Name:  "etcd",
		Usage: "etcd backup/restore operations in k8s cluster",
		Subcommands: []cli.Command{
			{
				Name:   "snapshot-save",
				Usage:  "Take snapshot on all etcd hosts",
				Flags:  backupRestoreFlags,
				Action: BackupEtcdHostsFromCli,
			},
			{
				Name:   "snapshot-restore",
				Usage:  "Restore existing snapshot",
				Flags:  backupRestoreFlags,
				Action: RestoreEtcdBackupFromCli,
			},
		},
	}
}

func BackupEtcdHosts(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory hosts.DialerFactory,
	configDir, backupName string) error {

	log.Infof(ctx, "Starting saving snapshot on etcd hosts")
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dockerDialerFactory, nil, nil)
	if err != nil {
		return err
	}

	if err := kubeCluster.TunnelHosts(ctx, false); err != nil {
		return err
	}
	if err := kubeCluster.BackupEtcd(ctx, backupName); err != nil {
		return err
	}

	log.Infof(ctx, "Finished saving snapshot on all etcd hosts")
	return nil
}

func RestoreEtcdBackup(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory hosts.DialerFactory,
	configDir, backupName string) error {

	log.Infof(ctx, "Starting restoring snapshot on etcd hosts")
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dockerDialerFactory, nil, nil)
	if err != nil {
		return err
	}

	if err := kubeCluster.TunnelHosts(ctx, false); err != nil {
		return err
	}
	if err := kubeCluster.RestoreEtcdBackup(ctx, backupName); err != nil {
		return err
	}

	log.Infof(ctx, "Finished restoring snapshot on all etcd hosts")
	return nil
}

func BackupEtcdHostsFromCli(ctx *cli.Context) error {
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return err
	}

	return BackupEtcdHosts(context.Background(), rkeConfig, nil, "", ctx.String("name"))
}

func RestoreEtcdBackupFromCli(ctx *cli.Context) error {
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return err
	}

	return RestoreEtcdBackup(context.Background(), rkeConfig, nil, "", ctx.String("name"))

}
