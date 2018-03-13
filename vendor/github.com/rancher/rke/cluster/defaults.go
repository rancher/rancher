package cluster

import (
	"context"

	ref "github.com/docker/distribution/reference"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	DefaultServiceClusterIPRange = "10.233.0.0/18"
	DefaultClusterCIDR           = "10.233.64.0/18"
	DefaultClusterDNSService     = "10.233.0.3"
	DefaultClusterDomain         = "cluster.local"
	DefaultClusterSSHKeyPath     = "~/.ssh/id_rsa"

	DefaultK8sVersion = v3.K8sV18

	DefaultSSHPort        = "22"
	DefaultDockerSockPath = "/var/run/docker.sock"

	DefaultAuthStrategy      = "x509"
	DefaultAuthorizationMode = "rbac"

	DefaultNetworkPlugin        = "flannel"
	DefaultNetworkCloudProvider = "none"

	DefaultIngressController = "nginx"
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

	c.setClusterImageDefaults()
	c.setClusterKubernetesImageVersion(ctx)
	c.setClusterServicesDefaults()
	c.setClusterNetworkDefaults()
}

func (c *Cluster) setClusterKubernetesImageVersion(ctx context.Context) {
	k8sImageNamed, _ := ref.ParseNormalizedNamed(c.SystemImages.Kubernetes)
	// Kubernetes image is already set by c.setClusterImageDefaults(),
	// I will override it here if Version is set.
	var VersionedImageNamed ref.NamedTagged
	if c.Version != "" {
		VersionedImageNamed, _ = ref.WithTag(ref.TrimNamed(k8sImageNamed), c.Version)
		c.SystemImages.Kubernetes = VersionedImageNamed.String()
	}
	normalizedSystemImage, _ := ref.ParseNormalizedNamed(c.SystemImages.Kubernetes)
	if normalizedSystemImage.String() != k8sImageNamed.String() {
		log.Infof(ctx, "Overrding Kubernetes image [%s] with tag [%s]", VersionedImageNamed.Name(), VersionedImageNamed.Tag())
	}
}

func (c *Cluster) setClusterServicesDefaults() {
	serviceConfigDefaultsMap := map[*string]string{
		&c.Services.KubeAPI.ServiceClusterIPRange:        DefaultServiceClusterIPRange,
		&c.Services.KubeController.ServiceClusterIPRange: DefaultServiceClusterIPRange,
		&c.Services.KubeController.ClusterCIDR:           DefaultClusterCIDR,
		&c.Services.Kubelet.ClusterDNSServer:             DefaultClusterDNSService,
		&c.Services.Kubelet.ClusterDomain:                DefaultClusterDomain,
		&c.Services.Kubelet.InfraContainerImage:          c.SystemImages.PodInfraContainer,
		&c.Authentication.Strategy:                       DefaultAuthStrategy,
		&c.Services.KubeAPI.Image:                        c.SystemImages.Kubernetes,
		&c.Services.Scheduler.Image:                      c.SystemImages.Kubernetes,
		&c.Services.KubeController.Image:                 c.SystemImages.Kubernetes,
		&c.Services.Kubelet.Image:                        c.SystemImages.Kubernetes,
		&c.Services.Kubeproxy.Image:                      c.SystemImages.Kubernetes,
		&c.Services.Etcd.Image:                           c.SystemImages.Etcd,
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
	switch c.Network.Plugin {

	case CalicoNetworkPlugin:
		networkPluginConfigDefaultsMap = map[string]string{
			CalicoCloudProvider: DefaultNetworkCloudProvider,
		}
	}
	for k, v := range networkPluginConfigDefaultsMap {
		setDefaultIfEmptyMapValue(c.Network.Options, k, v)
	}

}
