package cluster

import (
	"context"
	"fmt"
	"path"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (c *Cluster) BackupEtcd(ctx context.Context, backupName string) error {
	for _, host := range c.EtcdHosts {
		if err := services.RunEtcdBackup(ctx, host, c.PrivateRegistriesMap, c.SystemImages.Alpine, c.Services.Etcd.Creation, c.Services.Etcd.Retention, backupName, true); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) RestoreEtcdBackup(ctx context.Context, backupPath string) error {
	// Stopping all etcd containers
	for _, host := range c.EtcdHosts {
		if err := tearDownOldEtcd(ctx, host, c.SystemImages.Alpine, c.PrivateRegistriesMap); err != nil {
			return err
		}
	}
	// Start restore process on all etcd hosts
	initCluster := services.GetEtcdInitialCluster(c.EtcdHosts)
	for _, host := range c.EtcdHosts {
		if err := services.RestoreEtcdBackup(ctx, host, c.PrivateRegistriesMap, c.SystemImages.Etcd, backupPath, initCluster); err != nil {
			return fmt.Errorf("[etcd] Failed to restore etcd backup: %v", err)
		}
	}
	// Deploy Etcd Plane
	etcdNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
	// Build etcd node plan map
	for _, etcdHost := range c.EtcdHosts {
		etcdNodePlanMap[etcdHost.Address] = BuildRKEConfigNodePlan(ctx, c, etcdHost, etcdHost.DockerInfo)
	}
	etcdBackup := services.EtcdBackup{
		Backup:    c.Services.Etcd.Backup,
		Creation:  c.Services.Etcd.Creation,
		Retention: c.Services.Etcd.Retention,
	}
	if err := services.RunEtcdPlane(ctx, c.EtcdHosts, etcdNodePlanMap, c.LocalConnDialerFactory, c.PrivateRegistriesMap, c.UpdateWorkersOnly, c.SystemImages.Alpine, etcdBackup); err != nil {
		return fmt.Errorf("[etcd] Failed to bring up Etcd Plane: %v", err)
	}
	return nil
}

func tearDownOldEtcd(ctx context.Context, host *hosts.Host, cleanupImage string, prsMap map[string]v3.PrivateRegistry) error {
	if err := docker.DoRemoveContainer(ctx, host.DClient, services.EtcdContainerName, host.Address); err != nil {
		return fmt.Errorf("[etcd] Failed to stop old etcd containers: %v", err)
	}
	// cleanup etcd data directory
	toCleanPaths := []string{
		path.Join(host.PrefixPath, hosts.ToCleanEtcdDir),
	}
	return host.CleanUp(ctx, toCleanPaths, cleanupImage, prsMap)
}
