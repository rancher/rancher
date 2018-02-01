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

func runKubeproxy(ctx context.Context, host *hosts.Host, kubeproxyService v3.KubeproxyService, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry) error {
	imageCfg, hostCfg := buildKubeproxyConfig(host, kubeproxyService)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeproxyContainerName, host.Address, WorkerRole, prsMap); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeproxyPort, false, KubeproxyContainerName, df)
}

func removeKubeproxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeproxyContainerName, host.Address)
}

func buildKubeproxyConfig(host *hosts.Host, kubeproxyService v3.KubeproxyService) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: kubeproxyService.Image,
		Entrypoint: []string{"/opt/rke/entrypoint.sh",
			"kube-proxy",
			"--v=2",
			"--healthz-bind-address=0.0.0.0",
			"--kubeconfig=" + pki.GetConfigPath(pki.KubeProxyCertName),
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
		Privileged:    true,
	}
	for arg, value := range kubeproxyService.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		imageCfg.Entrypoint = append(imageCfg.Entrypoint, cmd)
	}
	return imageCfg, hostCfg
}
