package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runKubelet(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, kubeletProcess v3.Process, certMap map[string]pki.CertificatePKI, alpineImage string) error {
	imageCfg, hostCfg, healthCheckURL := GetProcessConfig(kubeletProcess, host)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeletContainerName, host.Address, WorkerRole, prsMap); err != nil {
		return err
	}
	if err := runHealthcheck(ctx, host, KubeletContainerName, df, healthCheckURL, certMap); err != nil {
		return err
	}
	return createLogLink(ctx, host, KubeletContainerName, WorkerRole, alpineImage, prsMap)
}

func removeKubelet(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeletContainerName, host.Address)
}

func RestartKubelet(ctx context.Context, host *hosts.Host) error {
	return docker.DoRestartContainer(ctx, host.DClient, KubeletContainerName, host.Address)
}
