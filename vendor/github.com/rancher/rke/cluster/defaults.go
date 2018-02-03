package cluster

import (
	"context"

	ref "github.com/docker/distribution/reference"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/services"
)

const (
	DefaultClusterConfig = "cluster.yml"

	DefaultServiceClusterIPRange = "10.233.0.0/18"
	DefaultClusterCIDR           = "10.233.64.0/18"
	DefaultClusterDNSService     = "10.233.0.3"
	DefaultClusterDomain         = "cluster.local"
	DefaultClusterSSHKeyPath     = "~/.ssh/id_rsa"

	DefaultDockerSockPath = "/var/run/docker.sock"

	DefaultAuthStrategy      = "x509"
	DefaultAuthorizationMode = "rbac"

	DefaultNetworkPlugin        = "flannel"
	DefaultNetworkCloudProvider = "none"

	DefaultInfraContainerImage            = "gcr.io/google_containers/pause-amd64:3.0"
	DefaultAplineImage                    = "alpine:latest"
	DefaultNginxProxyImage                = "rancher/rke-nginx-proxy:v0.1.1"
	DefaultCertDownloaderImage            = "rancher/rke-cert-deployer:v0.1.1"
	DefaultKubernetesServicesSidecarImage = "rancher/rke-service-sidekick:v0.1.0"
	DefaultIngressController              = "nginx"

	DefaultEtcdImage = "quay.io/coreos/etcd:v3.0.17"
	DefaultK8sImage  = "rancher/k8s:v1.8.5-rancher4"

	DefaultFlannelImage    = "quay.io/coreos/flannel:v0.9.1"
	DefaultFlannelCNIImage = "quay.io/coreos/flannel-cni:v0.2.0"

	DefaultCalicoNodeImage        = "quay.io/calico/node:v2.6.2"
	DefaultCalicoCNIImage         = "quay.io/calico/cni:v1.11.0"
	DefaultCalicoControllersImage = "quay.io/calico/kube-controllers:v1.0.0"
	DefaultCalicoctlImage         = "quay.io/calico/ctl:v1.6.2"

	DefaultWeaveImage    = "weaveworks/weave-kube:2.1.2"
	DefaultWeaveCNIImage = "weaveworks/weave-npc:2.1.2"

	DefaultCanalNodeImage    = "quay.io/calico/node:v2.6.2"
	DefaultCanalCNIImage     = "quay.io/calico/cni:v1.11.0"
	DefaultCanalFlannelImage = "quay.io/coreos/flannel:v0.9.1"

	DefaultKubeDNSImage           = "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5"
	DefaultDNSmasqImage           = "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5"
	DefaultKubeDNSSidecarImage    = "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5"
	DefaultKubeDNSAutoScalerImage = "gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"
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
	}
	if len(c.Authorization.Mode) == 0 {
		c.Authorization.Mode = DefaultAuthorizationMode
	}
	if c.Services.KubeAPI.PodSecurityPolicy && c.Authorization.Mode != services.RBACAuthorizationMode {
		log.Warnf(ctx, "PodSecurityPolicy can't be enabled with RBAC support disabled")
		c.Services.KubeAPI.PodSecurityPolicy = false
	}
	if len(c.Ingress.Type) == 0 {
		c.Ingress.Type = DefaultIngressController
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
		&c.Services.Kubelet.InfraContainerImage:          DefaultInfraContainerImage,
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

	systemImagesDefaultsMap := map[*string]string{
		&c.SystemImages.Alpine:                    DefaultAplineImage,
		&c.SystemImages.NginxProxy:                DefaultNginxProxyImage,
		&c.SystemImages.CertDownloader:            DefaultCertDownloaderImage,
		&c.SystemImages.KubeDNS:                   DefaultKubeDNSImage,
		&c.SystemImages.KubeDNSSidecar:            DefaultKubeDNSSidecarImage,
		&c.SystemImages.DNSmasq:                   DefaultDNSmasqImage,
		&c.SystemImages.KubeDNSAutoscaler:         DefaultKubeDNSAutoScalerImage,
		&c.SystemImages.KubernetesServicesSidecar: DefaultKubernetesServicesSidecarImage,
		&c.SystemImages.Etcd:                      DefaultEtcdImage,
		&c.SystemImages.Kubernetes:                DefaultK8sImage,
	}
	for k, v := range systemImagesDefaultsMap {
		setDefaultIfEmpty(k, v)
	}
}
