package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	NginxProxyImage   = "rancher/rke-nginx-proxy:0.1.0"
	NginxProxyEnvName = "CP_HOSTS"
)

func runNginxProxy(ctx context.Context, host *hosts.Host, prsMap map[string]v3.PrivateRegistry, proxyProcess v3.Process, alpineImage string) error {
	imageCfg, hostCfg, _ := GetProcessConfig(proxyProcess, host)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, NginxProxyContainerName, host.Address, WorkerRole, prsMap); err != nil {
		return err
	}
	return createLogLink(ctx, host, NginxProxyContainerName, WorkerRole, alpineImage, prsMap)
}

func removeNginxProxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, NginxProxyContainerName, host.Address)
}

func RestartNginxProxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRestartContainer(ctx, host.DClient, NginxProxyContainerName, host.Address)
}
