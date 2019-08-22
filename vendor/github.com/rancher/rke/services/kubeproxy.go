package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runKubeproxy(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, kubeProxyProcess v3.Process, alpineImage string) error {
	imageCfg, hostCfg, healthCheckURL := GetProcessConfig(kubeProxyProcess, host)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeproxyContainerName, host.Address, WorkerRole, prsMap); err != nil {
		return err
	}
	if err := runHealthcheck(ctx, host, KubeproxyContainerName, df, healthCheckURL, nil); err != nil {
		return err
	}
	return createLogLink(ctx, host, KubeproxyContainerName, WorkerRole, alpineImage, prsMap)
}

func removeKubeproxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeproxyContainerName, host.Address)
}

func RestartKubeproxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRestartContainer(ctx, host.DClient, KubeproxyContainerName, host.Address)
}
