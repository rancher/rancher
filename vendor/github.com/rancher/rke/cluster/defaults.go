package cluster

import (
	"context"

	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	DefaultServiceClusterIPRange = "10.43.0.0/16"
	DefaultNodePortRange         = "30000-32767"
	DefaultClusterCIDR           = "10.42.0.0/16"
	DefaultClusterDNSService     = "10.43.0.10"
	DefaultClusterDomain         = "cluster.local"
	DefaultClusterName           = "local"
	DefaultClusterSSHKeyPath     = "~/.ssh/id_rsa"

	DefaultK8sVersion = v3.DefaultK8s

	DefaultSSHPort        = "22"
	DefaultDockerSockPath = "/var/run/docker.sock"

	DefaultAuthStrategy      = "x509"
	DefaultAuthorizationMode = "rbac"

	DefaultNetworkPlugin        = "canal"
	DefaultNetworkCloudProvider = "none"

	DefaultIngressController         = "nginx"
	DefaultEtcdBackupCreationPeriod  = "5m0s"
	DefaultEtcdBackupRetentionPeriod = "24h"
)

func setDefaultIfEmptyMapValue(configMap map[string]string, key string, value string) {
	if _, ok := configMap[key]; !ok {
		configMap[key] = value
	}
}

func setDefaultIfEmpty(varName *string, defaultValue string) {
	if len(*varName) == 0 {
		*varName = defaultValue
	}
}

func (c *Cluster) setClusterDefaults(ctx context.Context) {
	if len(c.SSHKeyPath) == 0 {
		c.SSHKeyPath = DefaultClusterSSHKeyPath
	}
	// Default Path prefix
	if len(c.PrefixPath) == 0 {
		c.PrefixPath = "/"
	}
	// Set bastion/jump host defaults
	if len(c.BastionHost.Address) > 0 {
		if len(c.BastionHost.Port) == 0 {
			c.BastionHost.Port = DefaultSSHPort
		}
		if len(c.BastionHost.SSHKeyPath) == 0 {
			c.BastionHost.SSHKeyPath = c.SSHKeyPath
		}
		c.BastionHost.SSHAgentAuth = c.SSHAgentAuth

	}
	for i, host := range c.Nodes {
		if len(host.InternalAddress) == 0 {
			c.Nodes[i].InternalAddress = c.Nodes[i].Address
		}
		if len(host.HostnameOverride) == 0 {
			// This is a temporary modification
			c.Nodes[i].HostnameOverride = c.Nodes[i].Address
		}
		if len(host.SSHKeyPath) == 0 {
			c.Nodes[i].SSHKeyPath = c.SSHKeyPath
		}
		if len(host.Port) == 0 {
			c.Nodes[i].Port = DefaultSSHPort
		}

		// For now, you can set at the global level only.
		c.Nodes[i].SSHAgentAuth = c.SSHAgentAuth
	}

	if len(c.Authorization.Mode) == 0 {
		c.Authorization.Mode = DefaultAuthorizationMode
	}
	if c.Services.KubeAPI.PodSecurityPolicy && c.Authorization.Mode != services.RBACAuthorizationMode {
		log.Warnf(ctx, "PodSecurityPolicy can't be enabled with RBAC support disabled")
		c.Services.KubeAPI.PodSecurityPolicy = false
	}
	if len(c.Ingress.Provider) == 0 {
		c.Ingress.Provider = DefaultIngressController
	}
	if len(c.ClusterName) == 0 {
		c.ClusterName = DefaultClusterName
	}
	if len(c.Version) == 0 {
		c.Version = DefaultK8sVersion
	}
	if c.AddonJobTimeout == 0 {
		c.AddonJobTimeout = k8s.DefaultTimeout
	}
	c.setClusterImageDefaults()
	c.setClusterServicesDefaults()
	c.setClusterNetworkDefaults()
}

func (c *Cluster) setClusterServicesDefaults() {
	// We don't accept per service images anymore.
	c.Services.KubeAPI.Image = c.SystemImages.Kubernetes
	c.Services.Scheduler.Image = c.SystemImages.Kubernetes
	c.Services.KubeController.Image = c.SystemImages.Kubernetes
	c.Services.Kubelet.Image = c.SystemImages.Kubernetes
	c.Services.Kubeproxy.Image = c.SystemImages.Kubernetes
	c.Services.Etcd.Image = c.SystemImages.Etcd

	serviceConfigDefaultsMap := map[*string]string{
		&c.Services.KubeAPI.ServiceClusterIPRange:        DefaultServiceClusterIPRange,
		&c.Services.KubeAPI.ServiceNodePortRange:         DefaultNodePortRange,
		&c.Services.KubeController.ServiceClusterIPRange: DefaultServiceClusterIPRange,
		&c.Services.KubeController.ClusterCIDR:           DefaultClusterCIDR,
		&c.Services.Kubelet.ClusterDNSServer:             DefaultClusterDNSService,
		&c.Services.Kubelet.ClusterDomain:                DefaultClusterDomain,
		&c.Services.Kubelet.InfraContainerImage:          c.SystemImages.PodInfraContainer,
		&c.Authentication.Strategy:                       DefaultAuthStrategy,
		&c.Services.Etcd.Creation:                        DefaultEtcdBackupCreationPeriod,
		&c.Services.Etcd.Retention:                       DefaultEtcdBackupRetentionPeriod,
	}
	for k, v := range serviceConfigDefaultsMap {
		setDefaultIfEmpty(k, v)
	}
}

func (c *Cluster) setClusterImageDefaults() {
	imageDefaults, ok := v3.K8sVersionToRKESystemImages[c.Version]
	if !ok {
		imageDefaults = v3.K8sVersionToRKESystemImages[DefaultK8sVersion]
	}

	systemImagesDefaultsMap := map[*string]string{
		&c.SystemImages.Alpine:                    imageDefaults.Alpine,
		&c.SystemImages.NginxProxy:                imageDefaults.NginxProxy,
		&c.SystemImages.CertDownloader:            imageDefaults.CertDownloader,
		&c.SystemImages.KubeDNS:                   imageDefaults.KubeDNS,
		&c.SystemImages.KubeDNSSidecar:            imageDefaults.KubeDNSSidecar,
		&c.SystemImages.DNSmasq:                   imageDefaults.DNSmasq,
		&c.SystemImages.KubeDNSAutoscaler:         imageDefaults.KubeDNSAutoscaler,
		&c.SystemImages.KubernetesServicesSidecar: imageDefaults.KubernetesServicesSidecar,
		&c.SystemImages.Etcd:                      imageDefaults.Etcd,
		&c.SystemImages.Kubernetes:                imageDefaults.Kubernetes,
		&c.SystemImages.PodInfraContainer:         imageDefaults.PodInfraContainer,
		&c.SystemImages.Flannel:                   imageDefaults.Flannel,
		&c.SystemImages.FlannelCNI:                imageDefaults.FlannelCNI,
		&c.SystemImages.CalicoNode:                imageDefaults.CalicoNode,
		&c.SystemImages.CalicoCNI:                 imageDefaults.CalicoCNI,
		&c.SystemImages.CalicoCtl:                 imageDefaults.CalicoCtl,
		&c.SystemImages.CanalNode:                 imageDefaults.CanalNode,
		&c.SystemImages.CanalCNI:                  imageDefaults.CanalCNI,
		&c.SystemImages.CanalFlannel:              imageDefaults.CanalFlannel,
		&c.SystemImages.WeaveNode:                 imageDefaults.WeaveNode,
		&c.SystemImages.WeaveCNI:                  imageDefaults.WeaveCNI,
		&c.SystemImages.Ingress:                   imageDefaults.Ingress,
		&c.SystemImages.IngressBackend:            imageDefaults.IngressBackend,
	}

	for k, v := range systemImagesDefaultsMap {
		setDefaultIfEmpty(k, v)
	}
}

func (c *Cluster) setClusterNetworkDefaults() {
	setDefaultIfEmpty(&c.Network.Plugin, DefaultNetworkPlugin)

	if c.Network.Options == nil {
		// don't break if the user didn't define options
		c.Network.Options = make(map[string]string)
	}
	networkPluginConfigDefaultsMap := make(map[string]string)
	// This is still needed because RKE doesn't use c.Network.*NetworkProvider, that's a rancher type
	switch c.Network.Plugin {
	case CalicoNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			CalicoCloudProvider: DefaultNetworkCloudProvider,
		}
	}
	if c.Network.CalicoNetworkProvider != nil {
		setDefaultIfEmpty(&c.Network.CalicoNetworkProvider.CloudProvider, DefaultNetworkCloudProvider)
		networkPluginConfigDefaultsMap[CalicoCloudProvider] = c.Network.CalicoNetworkProvider.CloudProvider
	}
	if c.Network.FlannelNetworkProvider != nil {
		networkPluginConfigDefaultsMap[FlannelIface] = c.Network.FlannelNetworkProvider.Iface
	}
	if c.Network.CanalNetworkProvider != nil {
		networkPluginConfigDefaultsMap[CanalIface] = c.Network.CanalNetworkProvider.Iface
	}
	for k, v := range networkPluginConfigDefaultsMap {
		setDefaultIfEmptyMapValue(c.Network.Options, k, v)
	}
}
