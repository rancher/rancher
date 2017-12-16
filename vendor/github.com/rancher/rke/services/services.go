package services

import (
	"fmt"
	"net"

	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/sirupsen/logrus"
)

const (
	ETCDRole    = "etcd"
	ControlRole = "controlplane"
	WorkerRole  = "worker"

	SidekickServiceName   = "sidekick"
	RBACAuthorizationMode = "rbac"

	KubeAPIContainerName        = "kube-api"
	KubeletContainerName        = "kubelet"
	KubeproxyContainerName      = "kube-proxy"
	KubeControllerContainerName = "kube-controller"
	SchedulerContainerName      = "scheduler"
	EtcdContainerName           = "etcd"
	NginxProxyContainerName     = "nginx-proxy"
	SidekickContainerName       = "service-sidekick"
)

func GetKubernetesServiceIP(serviceClusterRange string) (net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(serviceClusterRange)
	if err != nil {
		return nil, fmt.Errorf("Failed to get kubernetes service IP from Kube API option [service_cluster_ip_range]: %v", err)
	}
	ip = ip.Mask(ipnet.Mask)
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
	return ip, nil
}

func buildSidekickConfig(sidekickImage string) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: sidekickImage,
	}
	hostCfg := &container.HostConfig{
		NetworkMode: "none",
	}
	return imageCfg, hostCfg
}

func runSidekick(host *hosts.Host, sidekickImage string) error {
	isRunning, err := docker.IsContainerRunning(host.DClient, host.Address, SidekickContainerName, true)
	if err != nil {
		return err
	}
	if isRunning {
		logrus.Infof("[%s] Sidekick container already created on host [%s]", SidekickServiceName, host.Address)
		return nil
	}
	imageCfg, hostCfg := buildSidekickConfig(sidekickImage)
	if err := docker.UseLocalOrPull(host.DClient, host.Address, sidekickImage, SidekickServiceName); err != nil {
		return err
	}
	if _, err := docker.CreateContiner(host.DClient, host.Address, SidekickContainerName, imageCfg, hostCfg); err != nil {
		return err
	}
	return nil
}

func removeSidekick(host *hosts.Host) error {
	return docker.DoRemoveContainer(host.DClient, SidekickContainerName, host.Address)
}
