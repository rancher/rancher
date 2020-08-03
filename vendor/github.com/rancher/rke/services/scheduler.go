package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	v3 "github.com/rancher/rke/types"
)

func runScheduler(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, schedulerProcess v3.Process, alpineImage string) error {
	imageCfg, hostCfg, healthCheckURL := GetProcessConfig(schedulerProcess, host)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, SchedulerContainerName, host.Address, ControlRole, prsMap); err != nil {
		return err
	}
	if err := runHealthcheck(ctx, host, SchedulerContainerName, df, healthCheckURL, nil); err != nil {
		return err
	}
	return createLogLink(ctx, host, SchedulerContainerName, ControlRole, alpineImage, prsMap)
}

func removeScheduler(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, SchedulerContainerName, host.Address)
}

func RestartScheduler(ctx context.Context, host *hosts.Host) error {
	return docker.DoRestartContainer(ctx, host.DClient, SchedulerContainerName, host.Address)
}
