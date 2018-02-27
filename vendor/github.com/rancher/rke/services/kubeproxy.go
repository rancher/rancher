package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runKubeproxy(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, kubeProxyProcess v3.Process) error {
	imageCfg, hostCfg, healthCheckURL := GetProcessConfig(kubeProxyProcess)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeproxyContainerName, host.Address, WorkerRole, prsMap); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeproxyContainerName, df, healthCheckURL)
}

func removeKubeproxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeproxyContainerName, host.Address)
}
