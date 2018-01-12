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

func runKubeController(ctx context.Context, host *hosts.Host, kubeControllerService v3.KubeControllerService, authorizationMode string, df hosts.DialerFactory) error {
	imageCfg, hostCfg := buildKubeControllerConfig(kubeControllerService, authorizationMode)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeControllerContainerName, host.Address, ControlRole); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeControllerPort, false, KubeControllerContainerName, df)
}

func removeKubeController(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeControllerContainerName, host.Address)
}

func buildKubeControllerConfig(kubeControllerService v3.KubeControllerService, authorizationMode string) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: kubeControllerService.Image,
		Entrypoint: []string{"/opt/rke/entrypoint.sh",
			"kube-controller-manager",
			"--address=0.0.0.0",
			"--cloud-provider=",
			"--leader-elect=true",
			"--kubeconfig=" + pki.KubeControllerConfigPath,
			"--enable-hostpath-provisioner=false",
			"--node-monitor-grace-period=40s",
			"--pod-eviction-timeout=5m0s",
			"--v=2",
			"--allocate-node-cidrs=true",
			"--cluster-cidr=" + kubeControllerService.ClusterCIDR,
			"--service-cluster-ip-range=" + kubeControllerService.ServiceClusterIPRange,
			"--service-account-private-key-file=" + pki.KubeAPIKeyPath,
			"--root-ca-file=" + pki.CACertPath,
		},
	}
	if authorizationMode == RBACAuthorizationMode {
		imageCfg.Cmd = append(imageCfg.Cmd, "--use-service-account-credentials=true")
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
	for arg, value := range kubeControllerService.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		imageCfg.Entrypoint = append(imageCfg.Entrypoint, cmd)
	}
	return imageCfg, hostCfg
}
