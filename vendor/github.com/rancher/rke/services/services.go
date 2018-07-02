package services

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	ETCDRole    = "etcd"
	ControlRole = "controlplane"
	WorkerRole  = "worker"

	SidekickServiceName   = "sidekick"
	RBACAuthorizationMode = "rbac"

	KubeAPIContainerName          = "kube-apiserver"
	KubeletContainerName          = "kubelet"
	KubeproxyContainerName        = "kube-proxy"
	KubeControllerContainerName   = "kube-controller-manager"
	SchedulerContainerName        = "kube-scheduler"
	EtcdContainerName             = "etcd"
	EtcdSnapshotContainerName     = "etcd-rolling-snapshots"
	EtcdSnapshotOnceContainerName = "etcd-snapshot-once"
	EtcdRestoreContainerName      = "etcd-restore"
	NginxProxyContainerName       = "nginx-proxy"
	SidekickContainerName         = "service-sidekick"
	LogLinkContainerName          = "rke-log-linker"
	LogCleanerContainerName       = "rke-log-cleaner"

	KubeAPIPort        = 6443
	SchedulerPort      = 10251
	KubeControllerPort = 10252
	KubeletPort        = 10250
	KubeproxyPort      = 10256
)

func runSidekick(ctx context.Context, host *hosts.Host, prsMap map[string]v3.PrivateRegistry, sidecarProcess v3.Process) error {
	isRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, SidekickContainerName, true)
	if err != nil {
		return err
	}
	imageCfg, hostCfg, _ := GetProcessConfig(sidecarProcess)
	isUpgradable := false
	if isRunning {
		isUpgradable, err = docker.IsContainerUpgradable(ctx, host.DClient, imageCfg, hostCfg, SidekickContainerName, host.Address, SidekickServiceName)
		if err != nil {
			return err
		}

		if !isUpgradable {
			log.Infof(ctx, "[%s] Sidekick container already created on host [%s]", SidekickServiceName, host.Address)
			return nil
		}
	}

	if err := docker.UseLocalOrPull(ctx, host.DClient, host.Address, sidecarProcess.Image, SidekickServiceName, prsMap); err != nil {
		return err
	}
	if isUpgradable {
		if err := docker.DoRemoveContainer(ctx, host.DClient, SidekickContainerName, host.Address); err != nil {
			return err
		}
	}
	if _, err := docker.CreateContainer(ctx, host.DClient, host.Address, SidekickContainerName, imageCfg, hostCfg); err != nil {
		return err
	}
	return nil
}

func removeSidekick(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, SidekickContainerName, host.Address)
}

func GetProcessConfig(process v3.Process) (*container.Config, *container.HostConfig, string) {
	imageCfg := &container.Config{
		Entrypoint: process.Command,
		Cmd:        process.Args,
		Env:        process.Env,
		Image:      process.Image,
		Labels:     process.Labels,
	}
	// var pidMode container.PidMode
	// pidMode = process.PidMode
	_, portBindings, _ := nat.ParsePortSpecs(process.Publish)
	hostCfg := &container.HostConfig{
		VolumesFrom:  process.VolumesFrom,
		Binds:        process.Binds,
		NetworkMode:  container.NetworkMode(process.NetworkMode),
		PidMode:      container.PidMode(process.PidMode),
		Privileged:   process.Privileged,
		PortBindings: portBindings,
	}
	if len(process.RestartPolicy) > 0 {
		hostCfg.RestartPolicy = container.RestartPolicy{Name: process.RestartPolicy}
	}
	return imageCfg, hostCfg, process.HealthCheck.URL
}

func GetHealthCheckURL(useTLS bool, port int) string {
	if useTLS {
		return fmt.Sprintf("%s%s:%d%s", HTTPSProtoPrefix, HealthzAddress, port, HealthzEndpoint)
	}
	return fmt.Sprintf("%s%s:%d%s", HTTPProtoPrefix, HealthzAddress, port, HealthzEndpoint)
}

func createLogLink(ctx context.Context, host *hosts.Host, containerName, plane, image string, prsMap map[string]v3.PrivateRegistry) error {
	logrus.Debugf("[%s] Creating log link for Container [%s] on host [%s]", plane, containerName, host.Address)
	containerInspect, err := docker.InspectContainer(ctx, host.DClient, host.Address, containerName)
	if err != nil {
		return err
	}
	containerID := containerInspect.ID
	containerLogPath := containerInspect.LogPath
	containerLogLink := fmt.Sprintf("%s/%s_%s.log", hosts.RKELogsPath, containerName, containerID)
	imageCfg := &container.Config{
		Image: image,
		Tty:   true,
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf("mkdir -p %s ; ln -s %s %s", hosts.RKELogsPath, containerLogPath, containerLogLink),
		},
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/var/lib:/var/lib",
		},
		Privileged: true,
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, LogLinkContainerName, host.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, LogLinkContainerName, host.Address, plane, prsMap); err != nil {
		return err
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, LogLinkContainerName, host.Address); err != nil {
		return err
	}
	logrus.Debugf("[%s] Successfully created log link for Container [%s] on host [%s]", plane, containerName, host.Address)
	return nil
}
