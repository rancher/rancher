package cluster

import (
	"fmt"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) TunnelHosts() error {
	for i := range c.EtcdHosts {
		if err := c.EtcdHosts[i].TunnelUp(c.DockerDialerFactory); err != nil {
			return fmt.Errorf("Failed to set up SSH tunneling for Etcd host [%s]: %v", c.EtcdHosts[i].Address, err)
		}
	}
	for i := range c.ControlPlaneHosts {
		err := c.ControlPlaneHosts[i].TunnelUp(c.DockerDialerFactory)
		if err != nil {
			return fmt.Errorf("Failed to set up SSH tunneling for Control host [%s]: %v", c.ControlPlaneHosts[i].Address, err)
		}
	}
	for i := range c.WorkerHosts {
		if err := c.WorkerHosts[i].TunnelUp(c.DockerDialerFactory); err != nil {
			return fmt.Errorf("Failed to set up SSH tunneling for Worker host [%s]: %v", c.WorkerHosts[i].Address, err)
		}
	}
	return nil
}

func (c *Cluster) InvertIndexHosts() error {
	c.EtcdHosts = make([]*hosts.Host, 0)
	c.WorkerHosts = make([]*hosts.Host, 0)
	c.ControlPlaneHosts = make([]*hosts.Host, 0)
	for _, host := range c.Nodes {
		newHost := hosts.Host{
			RKEConfigNode: host,
		}

		newHost.EnforceDockerVersion = c.EnforceDockerVersion

		for _, role := range host.Role {
			logrus.Debugf("Host: " + host.Address + " has role: " + role)
			switch role {
			case services.ETCDRole:
				c.EtcdHosts = append(c.EtcdHosts, &newHost)
			case services.ControlRole:
				newHost.IsControl = true
				c.ControlPlaneHosts = append(c.ControlPlaneHosts, &newHost)
			case services.WorkerRole:
				newHost.IsWorker = true
				c.WorkerHosts = append(c.WorkerHosts, &newHost)
			default:
				return fmt.Errorf("Failed to recognize host [%s] role %s", host.Address, role)
			}
		}
	}
	return nil
}

func (c *Cluster) SetUpHosts() error {
	if c.Authentication.Strategy == X509AuthenticationProvider {
		logrus.Infof("[certificates] Deploying kubernetes certificates to Cluster nodes")
		err := pki.DeployCertificatesOnMasters(c.ControlPlaneHosts, c.Certificates, c.SystemImages[CertDownloaderImage])
		if err != nil {
			return err
		}
		err = pki.DeployCertificatesOnWorkers(c.WorkerHosts, c.Certificates, c.SystemImages[CertDownloaderImage])
		if err != nil {
			return err
		}
		err = pki.DeployAdminConfig(c.Certificates[pki.KubeAdminCommonName].Config, c.LocalKubeConfigPath)
		if err != nil {
			return err
		}
		logrus.Infof("[certificates] Successfully deployed kubernetes certificates to Cluster nodes")
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
