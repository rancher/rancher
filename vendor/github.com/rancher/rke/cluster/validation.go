package cluster

import (
	"fmt"
	"strings"

	"github.com/rancher/rke/services"
)

func (c *Cluster) ValidateCluster() error {
	// make sure cluster has at least one controlplane/etcd host
	if err := ValidateHostCount(c); err != nil {
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
	if c.Authentication.Strategy != DefaultAuthStrategy {
		return fmt.Errorf("Authentication strategy [%s] is not supported", c.Authentication.Strategy)
	}
	return nil
}

func validateNetworkOptions(c *Cluster) error {
	if c.Network.Plugin != FlannelNetworkPlugin && c.Network.Plugin != CalicoNetworkPlugin && c.Network.Plugin != CanalNetworkPlugin && c.Network.Plugin != WeaveNetworkPlugin {
		return fmt.Errorf("Network plugin [%s] is not supported", c.Network.Plugin)
	}
	return nil
}

func validateHostsOptions(c *Cluster) error {
	for i, host := range c.Nodes {
		if len(host.Address) == 0 {
			return fmt.Errorf("User for host (%d) is not provided", i+1)
		}
		if len(host.User) == 0 {
			return fmt.Errorf("User for host (%d) is not provided", i+1)
		}
		if len(host.Role) == 0 {
			return fmt.Errorf("Role for host (%d) is not provided", i+1)
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
			return fmt.Errorf("External CA Certificate for etcd can't be empty")
		}
		if len(c.Services.Etcd.Cert) == 0 {
			return fmt.Errorf("External Client Certificate for etcd can't be empty")
		}
		if len(c.Services.Etcd.Key) == 0 {
			return fmt.Errorf("External Client Key for etcd can't be empty")
		}
		if len(c.Services.Etcd.Path) == 0 {
			return fmt.Errorf("External etcd path can't be empty")
		}
	}
	return nil
}

func validateIngressOptions(c *Cluster) error {
	// Should be changed when adding more ingress types
	if c.Ingress.Provider != DefaultIngressController && c.Ingress.Provider != "none" {
		return fmt.Errorf("Ingress controller %s is incorrect", c.Ingress.Provider)
	}
	return nil
}

func ValidateHostCount(c *Cluster) error {
	if len(c.EtcdHosts) == 0 && len(c.Services.Etcd.ExternalURLs) == 0 {
		return fmt.Errorf("Cluster must have at least one etcd plane host")
	}
	if len(c.EtcdHosts) > 0 && len(c.Services.Etcd.ExternalURLs) > 0 {
		return fmt.Errorf("Cluster can't have both internal and external etcd")
	}
	return nil
}

func validateDuplicateNodes(c *Cluster) error {
	for i := range c.Nodes {
		for j := range c.Nodes {
			if i == j {
				continue
			}
			if c.Nodes[i].Address == c.Nodes[j].Address {
				return fmt.Errorf("Cluster can't have duplicate node: %s", c.Nodes[i].Address)
			}
			if c.Nodes[i].HostnameOverride == c.Nodes[j].HostnameOverride {
				return fmt.Errorf("Cluster can't have duplicate node: %s", c.Nodes[i].HostnameOverride)
			}
		}
	}
	return nil
}
