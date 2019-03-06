package cluster

import (
	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/sync/errgroup"
)

func (c *Cluster) ClusterRemove(ctx context.Context) error {
	if err := c.CleanupNodes(ctx); err != nil {
		return err
	}
	c.CleanupFiles(ctx)
	return nil
}

func cleanUpHosts(ctx context.Context, cpHosts, workerHosts, etcdHosts []*hosts.Host, cleanerImage string, prsMap map[string]v3.PrivateRegistry, externalEtcd bool) error {

	uniqueHosts := hosts.GetUniqueHostList(cpHosts, workerHosts, etcdHosts)

	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(uniqueHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				if err := runHost.CleanUpAll(ctx, cleanerImage, prsMap, externalEtcd); err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}

	return errgrp.Wait()
}

func (c *Cluster) CleanupNodes(ctx context.Context) error {
	externalEtcd := false
	if len(c.Services.Etcd.ExternalURLs) > 0 {
		externalEtcd = true
	}
	// Remove Worker Plane
	if err := services.RemoveWorkerPlane(ctx, c.WorkerHosts, true); err != nil {
		return err
	}
	// Remove Contol Plane
	if err := services.RemoveControlPlane(ctx, c.ControlPlaneHosts, true); err != nil {
		return err
	}

	// Remove Etcd Plane
	if !externalEtcd {
		if err := services.RemoveEtcdPlane(ctx, c.EtcdHosts, true); err != nil {
			return err
		}
	}

	// Clean up all hosts
	return cleanUpHosts(ctx, c.ControlPlaneHosts, c.WorkerHosts, c.EtcdHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap, externalEtcd)
}

func (c *Cluster) CleanupFiles(ctx context.Context) error {
	pki.RemoveAdminConfig(ctx, c.LocalKubeConfigPath)
	removeStateFile(ctx, c.StateFilePath)
	return nil
}

func (c *Cluster) RemoveOldNodes(ctx context.Context) error {
	kubeClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return err
	}
	nodeList, err := k8s.GetNodeList(kubeClient)
	if err != nil {
		return err
	}
	uniqueHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	for _, node := range nodeList.Items {
		_, isEtcd := node.Labels[etcdRoleLabel]
		if k8s.IsNodeReady(node) && !isEtcd {
			continue
		}
		host := &hosts.Host{}
		host.HostnameOverride = node.Name
		if !hosts.IsNodeInList(host, uniqueHosts) {
			if err := k8s.DeleteNode(kubeClient, node.Name, c.CloudProvider.Name); err != nil {
				log.Warnf(ctx, "Failed to delete old node [%s] from kubernetes")
			}
		}
	}
	return nil
}
