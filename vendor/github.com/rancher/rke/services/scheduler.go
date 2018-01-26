package services

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runScheduler(ctx context.Context, host *hosts.Host, schedulerService v3.SchedulerService, df hosts.DialerFactory) error {
	imageCfg, hostCfg := buildSchedulerConfig(host, schedulerService)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, SchedulerContainerName, host.Address, ControlRole); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, SchedulerPort, false, SchedulerContainerName, df)
}

func removeScheduler(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, SchedulerContainerName, host.Address)
}

func buildSchedulerConfig(host *hosts.Host, schedulerService v3.SchedulerService) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: schedulerService.Image,
		Entrypoint: []string{"/opt/rke/entrypoint.sh",
			"kube-scheduler",
			"--leader-elect=true",
			"--v=2",
			"--address=0.0.0.0",
			"--kubeconfig=" + pki.GetConfigPath(pki.KubeSchedulerCertName),
		},
	}
	hostCfg := &container.HostConfig{
		VolumesFrom: []string{
			SidekickContainerName,
		},
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
		},
		NetworkMode:   "host",
		RestartPolicy: container.RestartPolicy{Name: "always"},
	}
	for arg, value := range schedulerService.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		imageCfg.Entrypoint = append(imageCfg.Entrypoint, cmd)
	}
	return imageCfg, hostCfg
}
