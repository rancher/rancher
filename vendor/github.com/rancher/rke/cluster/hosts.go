package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"sigs.k8s.io/yaml"
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

func (c *Cluster) CalculateMaxUnavailable() (int, int, error) {
	var inactiveControlPlaneHosts, inactiveWorkerHosts []string
	var workerHosts, controlHosts, maxUnavailableWorker, maxUnavailableControl int

	for _, host := range c.InactiveHosts {
		if host.IsControl {
			inactiveControlPlaneHosts = append(inactiveControlPlaneHosts, host.HostnameOverride)
		}
		if !host.IsWorker {
			inactiveWorkerHosts = append(inactiveWorkerHosts, host.HostnameOverride)
		}
		// not breaking out of the loop so we can log all of the inactive hosts
	}

	// maxUnavailable should be calculated against all hosts provided in cluster.yml
	workerHosts = len(c.WorkerHosts) + len(inactiveWorkerHosts)
	maxUnavailableWorker, err := services.CalculateMaxUnavailable(c.UpgradeStrategy.MaxUnavailableWorker, workerHosts, services.WorkerRole)
	if err != nil {
		return maxUnavailableWorker, maxUnavailableControl, err
	}
	controlHosts = len(c.ControlPlaneHosts) + len(inactiveControlPlaneHosts)
	maxUnavailableControl, err = services.CalculateMaxUnavailable(c.UpgradeStrategy.MaxUnavailableControlplane, controlHosts, services.ControlRole)
	if err != nil {
		return maxUnavailableWorker, maxUnavailableControl, err
	}
	return maxUnavailableWorker, maxUnavailableControl, nil
}

func (c *Cluster) getConsolidatedAdmissionConfiguration() (*v1alpha1.AdmissionConfiguration, error) {
	var err error
	var admissionConfig *v1alpha1.AdmissionConfiguration

	if c.Services.KubeAPI.EventRateLimit == nil ||
		!c.Services.KubeAPI.EventRateLimit.Enabled {
		return c.Services.KubeAPI.AdmissionConfiguration, nil
	}

	logrus.Debugf("EventRateLimit is enabled")
	found := false
	if c.Services.KubeAPI.AdmissionConfiguration != nil {
		plugins := c.Services.KubeAPI.AdmissionConfiguration.Plugins
		for _, plugin := range plugins {
			if plugin.Name == EventRateLimitPluginName {
				found = true
				break
			}
		}
	}
	if found {
		logrus.Debugf("EventRateLimit Plugin configuration found in admission config")
		if c.Services.KubeAPI.EventRateLimit.Configuration != nil {
			logrus.Warnf("conflicting EventRateLimit configuration found, using the one from Admission Configuration")
			return c.Services.KubeAPI.AdmissionConfiguration, nil
		}
	}

	logrus.Debugf("EventRateLimit Plugin configuration not found in admission config")
	if c.Services.KubeAPI.AdmissionConfiguration == nil {
		logrus.Debugf("no user specified admission configuration found")
		admissionConfig, err = newDefaultAdmissionConfiguration()
		if err != nil {
			logrus.Errorf("error getting default admission configuration: %v", err)
			return nil, err
		}
	} else {
		admissionConfig, err = newDefaultAdmissionConfiguration()
		if err != nil {
			logrus.Errorf("error getting default admission configuration: %v", err)
			return nil, err
		}
		copy(admissionConfig.Plugins, c.Services.KubeAPI.AdmissionConfiguration.Plugins)
	}
	if c.Services.KubeAPI.EventRateLimit.Configuration != nil {
		logrus.Debugf("user specified EventRateLimit configuration found")
		p, err := getEventRateLimitPluginFromConfig(c.Services.KubeAPI.EventRateLimit.Configuration)
		if err != nil {
			logrus.Errorf("error getting eventratelimit plugin from config: %v", err)
		}
		admissionConfig.Plugins = append(admissionConfig.Plugins, p)
	} else {
		logrus.Debugf("using default EventRateLimit configuration")
		p, err := newDefaultEventRateLimitPlugin()
		if err != nil {
			logrus.Errorf("error getting default eventratelimit plugin: %v", err)
		}
		admissionConfig.Plugins = append(admissionConfig.Plugins, p)
	}

	return admissionConfig, nil
}

func (c *Cluster) SetUpHosts(ctx context.Context, flags ExternalFlags) error {
	if c.AuthnStrategies[AuthnX509Provider] {
		log.Infof(ctx, "[certificates] Deploying kubernetes certificates to Cluster nodes")
		hostList := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
		var errgrp errgroup.Group
		hostsQueue := util.GetObjectQueue(hostList)
		for w := 0; w < WorkerThreads; w++ {
			errgrp.Go(func() error {
				var errList []error
				for host := range hostsQueue {
					err := pki.DeployCertificatesOnPlaneHost(ctx, host.(*hosts.Host), c.RancherKubernetesEngineConfig, c.Certificates, c.SystemImages.CertDownloader, c.PrivateRegistriesMap, c.ForceDeployCerts)
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
			log.Infof(ctx, "[%s] Successfully deployed authentication webhook config Cluster nodes", authnWebhookFileName)
		}
		if c.EncryptionConfig.EncryptionProviderFile != "" {
			if err := c.DeployEncryptionProviderFile(ctx); err != nil {
				return err
			}
		}

		if _, ok := c.Services.KubeAPI.ExtraArgs[KubeAPIArgAdmissionControlConfigFile]; !ok {
			if c.Services.KubeAPI.EventRateLimit != nil && c.Services.KubeAPI.EventRateLimit.Enabled {
				controlPlaneHosts := hosts.GetUniqueHostList(nil, c.ControlPlaneHosts, nil)
				ac, err := c.getConsolidatedAdmissionConfiguration()
				if err != nil {
					return fmt.Errorf("error getting consolidated admission configuration: %v", err)
				}
				bytes, err := yaml.Marshal(ac)
				if err != nil {
					return err
				}
				if err := deployFile(ctx, controlPlaneHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap, DefaultKubeAPIArgAdmissionControlConfigFileValue, string(bytes)); err != nil {
					return err
				}
				log.Infof(ctx, "[%s] Successfully deployed admission control config to Cluster control nodes", DefaultKubeAPIArgAdmissionControlConfigFileValue)
			}
		}

		if _, ok := c.Services.KubeAPI.ExtraArgs[KubeAPIArgAuditPolicyFile]; !ok {
			if c.Services.KubeAPI.AuditLog != nil && c.Services.KubeAPI.AuditLog.Enabled {
				controlPlaneHosts := hosts.GetUniqueHostList(nil, c.ControlPlaneHosts, nil)
				bytes, err := yaml.Marshal(c.Services.KubeAPI.AuditLog.Configuration.Policy)
				if err != nil {
					return err
				}
				if err := deployFile(ctx, controlPlaneHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap, DefaultKubeAPIArgAuditPolicyFileValue, string(bytes)); err != nil {
					return err
				}
				log.Infof(ctx, "[%s] Successfully deployed audit policy file to Cluster control nodes", DefaultKubeAPIArgAuditPolicyFileValue)
			}
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
	l := []v3.RKEConfigNode{}
	for _, node := range nodeList {
		if nodeToRemove.Address != node.Address {
			l = append(l, node)
		}
	}
	return l
}
