package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func EtcdCommand() cli.Command {
	snapshotFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "Specify Snapshot name",
		},
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  pki.ClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
	}

	snapshotFlags = append(snapshotFlags, commonFlags...)

	return cli.Command{
		Name:  "etcd",
		Usage: "etcd snapshot save/restore operations in k8s cluster",
		Subcommands: []cli.Command{
			{
				Name:   "snapshot-save",
				Usage:  "Take snapshot on all etcd hosts",
				Flags:  snapshotFlags,
				Action: SnapshotSaveEtcdHostsFromCli,
			},
			{
				Name:   "snapshot-restore",
				Usage:  "Restore existing snapshot",
				Flags:  snapshotFlags,
				Action: RestoreEtcdSnapshotFromCli,
			},
		},
	}
}

func SnapshotSaveEtcdHosts(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory hosts.DialerFactory,
	configDir, snapshotName string) error {

	log.Infof(ctx, "Starting saving snapshot on etcd hosts")
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dockerDialerFactory, nil, nil)
	if err != nil {
		return err
	}

	if err := kubeCluster.TunnelHosts(ctx, false); err != nil {
		return err
	}
	if err := kubeCluster.SnapshotEtcd(ctx, snapshotName); err != nil {
		return err
	}

	if err := kubeCluster.SaveBackupCertificateBundle(ctx); err != nil {
		return err
	}

	log.Infof(ctx, "Finished saving snapshot [%s] on all etcd hosts", snapshotName)
	return nil
}

func RestoreEtcdSnapshot(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory hosts.DialerFactory,
	configDir, snapshotName string) error {

	log.Infof(ctx, "Starting restoring snapshot on etcd hosts")
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dockerDialerFactory, nil, nil)
	if err != nil {
		return err
	}

	if err := kubeCluster.TunnelHosts(ctx, false); err != nil {
		return err
	}
	if err := kubeCluster.RestoreEtcdSnapshot(ctx, snapshotName); err != nil {
		return err
	}
	if err := kubeCluster.ExtractBackupCertificateBundle(ctx); err != nil {
		return err
	}
	log.Infof(ctx, "Finished restoring snapshot [%s] on all etcd hosts", snapshotName)
	return nil
}

func SnapshotSaveEtcdHostsFromCli(ctx *cli.Context) error {
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
	// Check snapshot name
	etcdSnapshotName := ctx.String("name")
	if etcdSnapshotName == "" {
		etcdSnapshotName = fmt.Sprintf("rke_etcd_snapshot_%s", time.Now().Format(time.RFC3339))
		logrus.Warnf("Name of the snapshot is not specified using [%s]", etcdSnapshotName)
	}
	return SnapshotSaveEtcdHosts(context.Background(), rkeConfig, nil, "", etcdSnapshotName)
}

func RestoreEtcdSnapshotFromCli(ctx *cli.Context) error {
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
	etcdSnapshotName := ctx.String("name")
	if etcdSnapshotName == "" {
		return fmt.Errorf("You must specify the snapshot name to restore")
	}
	return RestoreEtcdSnapshot(context.Background(), rkeConfig, nil, "", etcdSnapshotName)

}
