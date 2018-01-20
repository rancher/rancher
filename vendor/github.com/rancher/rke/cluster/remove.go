package cluster

import (
	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
)

func (c *Cluster) ClusterRemove(ctx context.Context) error {
	// Remove Worker Plane
	if err := services.RemoveWorkerPlane(ctx, c.WorkerHosts, true); err != nil {
		return err
	}

	// Remove Contol Plane
	if err := services.RemoveControlPlane(ctx, c.ControlPlaneHosts, true); err != nil {
		return err
	}

	// Remove Etcd Plane
	if err := services.RemoveEtcdPlane(ctx, c.EtcdHosts, true); err != nil {
		return err
	}

	// Clean up all hosts
	if err := cleanUpHosts(ctx, c.ControlPlaneHosts, c.WorkerHosts, c.EtcdHosts, c.SystemImages[AplineImage]); err != nil {
		return err
	}

	pki.RemoveAdminConfig(ctx, c.LocalKubeConfigPath)
	return nil
}

func cleanUpHosts(ctx context.Context, cpHosts, workerHosts, etcdHosts []*hosts.Host, cleanerImage string) error {
	allHosts := []*hosts.Host{}
	allHosts = append(allHosts, cpHosts...)
	allHosts = append(allHosts, workerHosts...)
	allHosts = append(allHosts, etcdHosts...)

	for _, host := range allHosts {
		if err := host.CleanUpAll(ctx, cleanerImage); err != nil {
			return err
		}
	}
	return nil
}
