package cluster

import (
	"fmt"
	"strings"

	"github.com/rancher/rke/services"
)

func (c *Cluster) ValidateCluster() error {
	// make sure cluster has at least one controlplane/etcd host
	if len(c.ControlPlaneHosts) == 0 {
		return fmt.Errorf("Cluster must have at least one control plane host")
	}
	if len(c.EtcdHosts)%2 == 0 {
		return fmt.Errorf("Cluster must have odd number of etcd nodes")
	}
	if len(c.WorkerHosts) == 0 {
		return fmt.Errorf("Cluster must have at least one worker plane host")
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
	return nil
}
