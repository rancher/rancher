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

func runKubelet(ctx context.Context, host *hosts.Host, kubeletService v3.KubeletService, df hosts.DialerFactory, unschedulable bool) error {
	imageCfg, hostCfg := buildKubeletConfig(host, kubeletService, unschedulable)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeletContainerName, host.Address, WorkerRole); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeletPort, true, KubeletContainerName, df)
}

func removeKubelet(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeletContainerName, host.Address)
}

func buildKubeletConfig(host *hosts.Host, kubeletService v3.KubeletService, unschedulable bool) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: kubeletService.Image,
		Entrypoint: []string{"/opt/rke/entrypoint.sh",
			"kubelet",
			"--v=2",
			"--address=0.0.0.0",
			"--cluster-domain=" + kubeletService.ClusterDomain,
			"--pod-infra-container-image=" + kubeletService.InfraContainerImage,
			"--cgroups-per-qos=True",
			"--enforce-node-allocatable=",
			"--hostname-override=" + host.HostnameOverride,
			"--cluster-dns=" + kubeletService.ClusterDNSServer,
			"--network-plugin=cni",
			"--cni-conf-dir=/etc/cni/net.d",
			"--cni-bin-dir=/opt/cni/bin",
			"--resolv-conf=/etc/resolv.conf",
			"--allow-privileged=true",
			"--cloud-provider=",
			"--kubeconfig=" + pki.GetConfigPath(pki.KubeNodeCertName),
			"--require-kubeconfig=True",
		},
	}
	if unschedulable {
		imageCfg.Cmd = append(imageCfg.Cmd, "--register-with-taints=node-role.kubernetes.io/etcd=true:NoSchedule")
	}
	for _, role := range host.Role {
		switch role {
		case ETCDRole:
			imageCfg.Cmd = append(imageCfg.Cmd, "--node-labels=node-role.kubernetes.io/etcd=true")
		case ControlRole:
			imageCfg.Cmd = append(imageCfg.Cmd, "--node-labels=node-role.kubernetes.io/master=true")
		case WorkerRole:
			imageCfg.Cmd = append(imageCfg.Cmd, "--node-labels=node-role.kubernetes.io/worker=true")
		}
	}
	hostCfg := &container.HostConfig{
		VolumesFrom: []string{
			SidekickContainerName,
		},
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
			"/usr/libexec/kubernetes/kubelet-plugins:/usr/libexec/kubernetes/kubelet-plugins",
			"/etc/cni:/etc/cni:ro",
			"/opt/cni:/opt/cni:ro",
			"/etc/resolv.conf:/etc/resolv.conf",
			"/sys:/sys",
			"/var/lib/docker:/var/lib/docker:rw",
			"/var/lib/kubelet:/var/lib/kubelet:shared",
			"/var/run:/var/run:rw",
			"/run:/run",
			"/etc/ceph:/etc/ceph",
			"/dev:/host/dev",
			"/var/log/containers:/var/log/containers",
			"/var/log/pods:/var/log/pods"},
		NetworkMode:   "host",
		PidMode:       "host",
		Privileged:    true,
		RestartPolicy: container.RestartPolicy{Name: "always"},
	}
	for arg, value := range kubeletService.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		imageCfg.Entrypoint = append(imageCfg.Entrypoint, cmd)
	}
	return imageCfg, hostCfg
}
