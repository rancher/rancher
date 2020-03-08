package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/rke/log"
	"github.com/rancher/rke/metadata"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func (c *Cluster) ValidateCluster(ctx context.Context) error {
	// validate kubernetes version
	// Version Check
	if err := validateVersion(ctx, c); err != nil {
		return err
	}

	// validate duplicate nodes
	if err := validateDuplicateNodes(c); err != nil {
		return err
	}

	// validate hosts options
	if err := validateHostsOptions(c); err != nil {
		return err
	}

	// validate Auth options
	if err := validateAuthOptions(c); err != nil {
		return err
	}

	// validate Network options
	if err := validateNetworkOptions(c); err != nil {
		return err
	}

	// validate Ingress options
	if err := validateIngressOptions(c); err != nil {
		return err
	}

	// validate services options
	return validateServicesOptions(c)
}

func validateAuthOptions(c *Cluster) error {
	for strategy, enabled := range c.AuthnStrategies {
		if !enabled {
			continue
		}
		strategy = strings.ToLower(strategy)
		if strategy != AuthnX509Provider && strategy != AuthnWebhookProvider {
			return fmt.Errorf("Authentication strategy [%s] is not supported", strategy)
		}
	}
	if !c.AuthnStrategies[AuthnX509Provider] {
		return fmt.Errorf("Authentication strategy must contain [%s]", AuthnX509Provider)
	}
	return nil
}

func validateNetworkOptions(c *Cluster) error {
	if c.Network.Plugin != NoNetworkPlugin && c.Network.Plugin != FlannelNetworkPlugin && c.Network.Plugin != CalicoNetworkPlugin && c.Network.Plugin != CanalNetworkPlugin && c.Network.Plugin != WeaveNetworkPlugin {
		return fmt.Errorf("Network plugin [%s] is not supported", c.Network.Plugin)
	}
	if c.Network.Plugin == FlannelNetworkPlugin && c.Network.MTU != 0 {
		return fmt.Errorf("Network plugin [%s] does not support configuring MTU", FlannelNetworkPlugin)
	}
	return nil
}

func validateHostsOptions(c *Cluster) error {
	for i, host := range c.Nodes {
		if len(host.Address) == 0 {
			return fmt.Errorf("Address for host (%d) is not provided", i+1)
		}
		if len(host.User) == 0 {
			return fmt.Errorf("User for host (%d) is not provided", i+1)
		}
		if len(host.Role) == 0 {
			return fmt.Errorf("Role for host (%d) is not provided", i+1)
		}
		if errs := validation.IsDNS1123Subdomain(host.HostnameOverride); len(errs) > 0 {
			return fmt.Errorf("Hostname_override [%s] for host (%d) is not valid: %v", host.HostnameOverride, i+1, errs)
		}
		for _, role := range host.Role {
			if role != services.ETCDRole && role != services.ControlRole && role != services.WorkerRole {
				return fmt.Errorf("Role [%s] for host (%d) is not recognized", role, i+1)
			}
		}
	}
	return nil
}

func validateServicesOptions(c *Cluster) error {
	servicesOptions := map[string]string{
		"etcd_image":                               c.Services.Etcd.Image,
		"kube_api_image":                           c.Services.KubeAPI.Image,
		"kube_api_service_cluster_ip_range":        c.Services.KubeAPI.ServiceClusterIPRange,
		"kube_controller_image":                    c.Services.KubeController.Image,
		"kube_controller_service_cluster_ip_range": c.Services.KubeController.ServiceClusterIPRange,
		"kube_controller_cluster_cidr":             c.Services.KubeController.ClusterCIDR,
		"scheduler_image":                          c.Services.Scheduler.Image,
		"kubelet_image":                            c.Services.Kubelet.Image,
		"kubelet_cluster_dns_service":              c.Services.Kubelet.ClusterDNSServer,
		"kubelet_cluster_domain":                   c.Services.Kubelet.ClusterDomain,
		"kubelet_infra_container_image":            c.Services.Kubelet.InfraContainerImage,
		"kubeproxy_image":                          c.Services.Kubeproxy.Image,
	}
	for optionName, OptionValue := range servicesOptions {
		if len(OptionValue) == 0 {
			return fmt.Errorf("%s can't be empty", strings.Join(strings.Split(optionName, "_"), " "))
		}
	}
	// Validate external etcd information
	if len(c.Services.Etcd.ExternalURLs) > 0 {
		if len(c.Services.Etcd.CACert) == 0 {
			return errors.New("External CA Certificate for etcd can't be empty")
		}
		if len(c.Services.Etcd.Cert) == 0 {
			return errors.New("External Client Certificate for etcd can't be empty")
		}
		if len(c.Services.Etcd.Key) == 0 {
			return errors.New("External Client Key for etcd can't be empty")
		}
		if len(c.Services.Etcd.Path) == 0 {
			return errors.New("External etcd path can't be empty")
		}
	}

	// validate etcd s3 backup backend configurations
	if err := validateEtcdBackupOptions(c); err != nil {
		return err
	}

	return nil
}

func validateEtcdBackupOptions(c *Cluster) error {
	if c.Services.Etcd.BackupConfig != nil {
		if c.Services.Etcd.BackupConfig.S3BackupConfig != nil {
			if len(c.Services.Etcd.BackupConfig.S3BackupConfig.Endpoint) == 0 {
				return errors.New("etcd s3 backup backend endpoint can't be empty")
			}
			if len(c.Services.Etcd.BackupConfig.S3BackupConfig.BucketName) == 0 {
				return errors.New("etcd s3 backup backend bucketName can't be empty")
			}
			if len(c.Services.Etcd.BackupConfig.S3BackupConfig.CustomCA) != 0 {
				if isValid, err := pki.IsValidCertStr(c.Services.Etcd.BackupConfig.S3BackupConfig.CustomCA); !isValid {
					return fmt.Errorf("invalid S3 endpoint CA certificate: %v", err)
				}
			}
		}
	}
	return nil
}

func validateIngressOptions(c *Cluster) error {
	// Should be changed when adding more ingress types
	if c.Ingress.Provider != DefaultIngressController && c.Ingress.Provider != "none" {
		return fmt.Errorf("Ingress controller %s is incorrect", c.Ingress.Provider)
	}

	if c.Ingress.DNSPolicy != "" &&
		!(c.Ingress.DNSPolicy == string(v1.DNSClusterFirst) ||
			c.Ingress.DNSPolicy == string(v1.DNSClusterFirstWithHostNet) ||
			c.Ingress.DNSPolicy == string(v1.DNSNone) ||
			c.Ingress.DNSPolicy == string(v1.DNSDefault)) {
		return fmt.Errorf("DNSPolicy %s was not a valid DNS Policy", c.Ingress.DNSPolicy)
	}

	return nil
}

func ValidateHostCount(c *Cluster) error {
	if len(c.EtcdHosts) == 0 && len(c.Services.Etcd.ExternalURLs) == 0 {
		failedEtcdHosts := []string{}
		for _, host := range c.InactiveHosts {
			if host.IsEtcd {
				failedEtcdHosts = append(failedEtcdHosts, host.Address)
			}
			return fmt.Errorf("Cluster must have at least one etcd plane host: failed to connect to the following etcd host(s) %v", failedEtcdHosts)
		}
		return errors.New("Cluster must have at least one etcd plane host: please specify one or more etcd in cluster config")
	}
	if len(c.EtcdHosts) > 0 && len(c.Services.Etcd.ExternalURLs) > 0 {
		return errors.New("Cluster can't have both internal and external etcd")
	}
	return nil
}

func validateDuplicateNodes(c *Cluster) error {
	addresses := make(map[string]struct{}, len(c.Nodes))
	hostnames := make(map[string]struct{}, len(c.Nodes))
	for i := range c.Nodes {
		if _, ok := addresses[c.Nodes[i].Address]; ok {
			return fmt.Errorf("Cluster can't have duplicate node: %s", c.Nodes[i].Address)
		}
		addresses[c.Nodes[i].Address] = struct{}{}
		if _, ok := hostnames[c.Nodes[i].HostnameOverride]; ok {
			return fmt.Errorf("Cluster can't have duplicate node: %s", c.Nodes[i].HostnameOverride)
		}
		hostnames[c.Nodes[i].HostnameOverride] = struct{}{}
	}
	return nil
}

func validateVersion(ctx context.Context, c *Cluster) error {
	_, err := util.StrToSemVer(c.Version)
	if err != nil {
		return fmt.Errorf("%s is not valid semver", c.Version)
	}
	_, ok := metadata.K8sVersionToRKESystemImages[c.Version]
	if !ok {
		if err := validateSystemImages(c); err != nil {
			return fmt.Errorf("%s is an unsupported Kubernetes version and system images are not populated: %v", c.Version, err)
		}
		return nil
	}

	if _, ok := metadata.K8sBadVersions[c.Version]; ok {
		log.Warnf(ctx, "%s version exists but its recommended to install this version - see 'rke config --system-images --all' for versions supported with this release", c.Version)
		return fmt.Errorf("%s is an unsupported Kubernetes version and system images are not populated: %v", c.Version, err)
	}

	return nil
}

func validateSystemImages(c *Cluster) error {
	if err := validateKubernetesImages(c); err != nil {
		return err
	}
	if err := validateNetworkImages(c); err != nil {
		return err
	}
	if err := validateDNSImages(c); err != nil {
		return err
	}
	if err := validateMetricsImages(c); err != nil {
		return err
	}
	if err := validateIngressImages(c); err != nil {
		return err
	}
	return nil
}

func validateKubernetesImages(c *Cluster) error {
	if len(c.SystemImages.Etcd) == 0 {
		return errors.New("etcd image is not populated")
	}
	if len(c.SystemImages.Kubernetes) == 0 {
		return errors.New("kubernetes image is not populated")
	}
	if len(c.SystemImages.PodInfraContainer) == 0 {
		return errors.New("pod infrastructure container image is not populated")
	}
	if len(c.SystemImages.Alpine) == 0 {
		return errors.New("alpine image is not populated")
	}
	if len(c.SystemImages.NginxProxy) == 0 {
		return errors.New("nginx proxy image is not populated")
	}
	if len(c.SystemImages.CertDownloader) == 0 {
		return errors.New("certificate downloader image is not populated")
	}
	if len(c.SystemImages.KubernetesServicesSidecar) == 0 {
		return errors.New("kubernetes sidecar image is not populated")
	}
	return nil
}

func validateNetworkImages(c *Cluster) error {
	// check network provider images
	if c.Network.Plugin == FlannelNetworkPlugin {
		if len(c.SystemImages.Flannel) == 0 {
			return errors.New("flannel image is not populated")
		}
		if len(c.SystemImages.FlannelCNI) == 0 {
			return errors.New("flannel cni image is not populated")
		}
	} else if c.Network.Plugin == CanalNetworkPlugin {
		if len(c.SystemImages.CanalNode) == 0 {
			return errors.New("canal image is not populated")
		}
		if len(c.SystemImages.CanalCNI) == 0 {
			return errors.New("canal cni image is not populated")
		}
		if len(c.SystemImages.CanalFlannel) == 0 {
			return errors.New("flannel image is not populated")
		}
	} else if c.Network.Plugin == CalicoNetworkPlugin {
		if len(c.SystemImages.CalicoCNI) == 0 {
			return errors.New("calico cni image is not populated")
		}
		if len(c.SystemImages.CalicoCtl) == 0 {
			return errors.New("calico ctl image is not populated")
		}
		if len(c.SystemImages.CalicoNode) == 0 {
			return errors.New("calico image is not populated")
		}
		if len(c.SystemImages.CalicoControllers) == 0 {
			return errors.New("calico controllers image is not populated")
		}
	} else if c.Network.Plugin == WeaveNetworkPlugin {
		if len(c.SystemImages.WeaveCNI) == 0 {
			return errors.New("weave cni image is not populated")
		}
		if len(c.SystemImages.WeaveNode) == 0 {
			return errors.New("weave image is not populated")
		}
	}
	return nil
}

func validateDNSImages(c *Cluster) error {
	// check dns provider images
	if c.DNS.Provider == "kube-dns" {
		if len(c.SystemImages.KubeDNS) == 0 {
			return errors.New("kubedns image is not populated")
		}
		if len(c.SystemImages.DNSmasq) == 0 {
			return errors.New("dnsmasq image is not populated")
		}
		if len(c.SystemImages.KubeDNSSidecar) == 0 {
			return errors.New("kubedns sidecar image is not populated")
		}
		if len(c.SystemImages.KubeDNSAutoscaler) == 0 {
			return errors.New("kubedns autoscaler image is not populated")
		}
	} else if c.DNS.Provider == "coredns" {
		if len(c.SystemImages.CoreDNS) == 0 {
			return errors.New("coredns image is not populated")
		}
		if len(c.SystemImages.CoreDNSAutoscaler) == 0 {
			return errors.New("coredns autoscaler image is not populated")
		}
	}
	if c.DNS.Nodelocal != nil && len(c.SystemImages.Nodelocal) == 0 {
		return errors.New("nodelocal image is not populated")
	}
	return nil
}

func validateMetricsImages(c *Cluster) error {
	// checl metrics server image
	if c.Monitoring.Provider != "none" {
		if len(c.SystemImages.MetricsServer) == 0 {
			return errors.New("metrics server images is not populated")
		}
	}
	return nil
}

func validateIngressImages(c *Cluster) error {
	// check ingress images
	if c.Ingress.Provider != "none" {
		if len(c.SystemImages.Ingress) == 0 {
			return errors.New("ingress image is not populated")
		}
		if len(c.SystemImages.IngressBackend) == 0 {
			return errors.New("ingress backend image is not populated")
		}
	}
	return nil
}
