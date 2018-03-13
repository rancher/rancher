package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runKubeController(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, controllerProcess v3.Process) error {
	imageCfg, hostCfg, healthCheckURL := GetProcessConfig(controllerProcess)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeControllerContainerName, host.Address, ControlRole, prsMap); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeControllerContainerName, df, healthCheckURL, nil)
}

func removeKubeController(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeControllerContainerName, host.Address)
}
