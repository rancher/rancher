package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runKubelet(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, kubeletProcess v3.Process) error {
	imageCfg, hostCfg, healthCheckURL := getProcessConfig(kubeletProcess)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeletContainerName, host.Address, WorkerRole, prsMap); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeletContainerName, df, healthCheckURL)
}

func removeKubelet(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeletContainerName, host.Address)
}
