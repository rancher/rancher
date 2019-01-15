package cluster

import (
	"fmt"
	"strings"

	"context"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	etcdRoleLabel         = "node-role.kubernetes.io/etcd"
	controlplaneRoleLabel = "node-role.kubernetes.io/controlplane"
	workerRoleLabel       = "node-role.kubernetes.io/worker"
	cloudConfigFileName   = "/etc/kubernetes/cloud-config"
	authnWebhookFileName  = "/etc/kubernetes/kube-api-authn-webhook.yaml"
)

func (c *Cluster) TunnelHosts(ctx context.Context, flags ExternalFlags) error {
	if flags.Local {
		if err := c.ControlPlaneHosts[0].TunnelUpLocal(ctx, c.Version); err != nil {
			return fmt.Errorf("Failed to connect to docker for local host [%s]: %v", c.EtcdHosts[0].Address, err)
		}
		return nil
	}
	c.InactiveHosts = make([]*hosts.Host, 0)
	uniqueHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
	var errgrp errgroup.Group
	for _, uniqueHost := range uniqueHosts {
		runHost := uniqueHost
		errgrp.Go(func() error {
			if err := runHost.TunnelUp(ctx, c.DockerDialerFactory, c.PrefixPath, c.Version); err != nil {
				// Unsupported Docker version is NOT a connectivity problem that we can recover! So we bail out on it
				if strings.Contains(err.Error(), "Unsupported Docker version found") {
					return err
				}
				log.Warnf(ctx, "Failed to set up SSH tunneling for host [%s]: %v", runHost.Address, err)
				c.InactiveHosts = append(c.InactiveHosts, runHost)
			}
			return nil
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	for _, host := range c.InactiveHosts {
		log.Warnf(ctx, "Removing host [%s] from node lists", host.Address)
		c.EtcdHosts = removeFromHosts(host, c.EtcdHosts)
		c.ControlPlaneHosts = removeFromHosts(host, c.ControlPlaneHosts)
		c.WorkerHosts = removeFromHosts(host, c.WorkerHosts)
		c.RancherKubernetesEngineConfig.Nodes = removeFromRKENodes(host.RKEConfigNode, c.RancherKubernetesEngineConfig.Nodes)
	}
	return ValidateHostCount(c)

}

func (c *Cluster) InvertIndexHosts() error {
	c.EtcdHosts = make([]*hosts.Host, 0)
	c.WorkerHosts = make([]*hosts.Host, 0)
	c.ControlPlaneHosts = make([]*hosts.Host, 0)
	for _, host := range c.Nodes {
		newHost := hosts.Host{
			RKEConfigNode: host,
			ToAddLabels:   map[string]string{},
			ToDelLabels:   map[string]string{},
			ToAddTaints:   []string{},
			ToDelTaints:   []string{},
			DockerInfo: types.Info{
				DockerRootDir: "/var/lib/docker",
			},
		}
		for k, v := range host.Labels {
			newHost.ToAddLabels[k] = v
		}
		newHost.IgnoreDockerVersion = c.IgnoreDockerVersion
		if c.BastionHost.Address != "" {
			// Add the bastion host information to each host object
			newHost.BastionHost = c.BastionHost
		}
		for _, role := range host.Role {
			logrus.Debugf("Host: " + host.Address + " has role: " + role)
			switch role {
			case services.ETCDRole:
				newHost.IsEtcd = true
				newHost.ToAddLabels[etcdRoleLabel] = "true"
				c.EtcdHosts = append(c.EtcdHosts, &newHost)
			case services.ControlRole:
				newHost.IsControl = true
				newHost.ToAddLabels[controlplaneRoleLabel] = "true"
				c.ControlPlaneHosts = append(c.ControlPlaneHosts, &newHost)
			case services.WorkerRole:
				newHost.IsWorker = true
				newHost.ToAddLabels[workerRoleLabel] = "true"
				c.WorkerHosts = append(c.WorkerHosts, &newHost)
			default:
				return fmt.Errorf("Failed to recognize host [%s] role %s", host.Address, role)
			}
		}
		if !newHost.IsEtcd {
			newHost.ToDelLabels[etcdRoleLabel] = "true"
		}
		if !newHost.IsControl {
			newHost.ToDelLabels[controlplaneRoleLabel] = "true"
		}
		if !newHost.IsWorker {
			newHost.ToDelLabels[workerRoleLabel] = "true"
		}
	}
	return nil
}

func (c *Cluster) SetUpHosts(ctx context.Context, flags ExternalFlags) error {
	if c.AuthnStrategies[AuthnX509Provider] {
		log.Infof(ctx, "[certificates] Deploying kubernetes certificates to Cluster nodes")
		forceDeploy := false
		if flags.CustomCerts || c.RancherKubernetesEngineConfig.RotateCertificates != nil {
			forceDeploy = true
		}
		hostList := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
		var errgrp errgroup.Group

		hostsQueue := util.GetObjectQueue(hostList)
		for w := 0; w < WorkerThreads; w++ {
			errgrp.Go(func() error {
				var errList []error
				for host := range hostsQueue {
					err := pki.DeployCertificatesOnPlaneHost(ctx, host.(*hosts.Host), c.RancherKubernetesEngineConfig, c.Certificates, c.SystemImages.CertDownloader, c.PrivateRegistriesMap, forceDeploy)
					if err != nil {
						errList = append(errList, err)
					}
				}
				return util.ErrList(errList)
			})
		}
		if err := errgrp.Wait(); err != nil {
			return err
		}

		if err := rebuildLocalAdminConfig(ctx, c); err != nil {
			return err
		}
		log.Infof(ctx, "[certificates] Successfully deployed kubernetes certificates to Cluster nodes")
		if c.CloudProvider.Name != "" {
			if err := deployFile(ctx, hostList, c.SystemImages.Alpine, c.PrivateRegistriesMap, cloudConfigFileName, c.CloudConfigFile); err != nil {
				return err
			}
			log.Infof(ctx, "[%s] Successfully deployed kubernetes cloud config to Cluster nodes", cloudConfigFileName)
		}

		if c.Authentication.Webhook != nil {
			if err := deployFile(ctx, hostList, c.SystemImages.Alpine, c.PrivateRegistriesMap, authnWebhookFileName, c.Authentication.Webhook.ConfigFile); err != nil {
				return err
			}
			log.Infof(ctx, "[%s] Successfully deployed authentication webhook config Cluster nodes", cloudConfigFileName)
		}
	}
	return nil
}

func CheckEtcdHostsChanged(kubeCluster, currentCluster *Cluster) error {
	if currentCluster != nil {
		etcdChanged := hosts.IsHostListChanged(currentCluster.EtcdHosts, kubeCluster.EtcdHosts)
		if etcdChanged {
			return fmt.Errorf("Adding or removing Etcd nodes is not supported")
		}
	}
	return nil
}

func removeFromHosts(hostToRemove *hosts.Host, hostList []*hosts.Host) []*hosts.Host {
	for i := range hostList {
		if hostToRemove.Address == hostList[i].Address {
			return append(hostList[:i], hostList[i+1:]...)
		}
	}
	return hostList
}

func removeFromRKENodes(nodeToRemove v3.RKEConfigNode, nodeList []v3.RKEConfigNode) []v3.RKEConfigNode {
	for i := range nodeList {
		if nodeToRemove.Address == nodeList[i].Address {
			return append(nodeList[:i], nodeList[i+1:]...)
		}
	}
	return nodeList
}
