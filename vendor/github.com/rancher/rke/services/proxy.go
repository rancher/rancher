package services

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
)

const (
	NginxProxyImage   = "rancher/rke-nginx-proxy:0.1.0"
	NginxProxyEnvName = "CP_HOSTS"
)

func RollingUpdateNginxProxy(ctx context.Context, cpHosts []*hosts.Host, workerHosts []*hosts.Host, nginxProxyImage string) error {
	nginxProxyEnv := buildProxyEnv(cpHosts)
	for _, host := range workerHosts {
		imageCfg, hostCfg := buildNginxProxyConfig(host, nginxProxyEnv, nginxProxyImage)
		if err := docker.DoRollingUpdateContainer(ctx, host.DClient, imageCfg, hostCfg, NginxProxyContainerName, host.Address, WorkerRole); err != nil {
			return err
		}
	}
	return nil
}

func runNginxProxy(ctx context.Context, host *hosts.Host, cpHosts []*hosts.Host, nginxProxyImage string) error {
	nginxProxyEnv := buildProxyEnv(cpHosts)
	imageCfg, hostCfg := buildNginxProxyConfig(host, nginxProxyEnv, nginxProxyImage)
	return docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, NginxProxyContainerName, host.Address, WorkerRole)
}

func removeNginxProxy(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, NginxProxyContainerName, host.Address)
}

func buildNginxProxyConfig(host *hosts.Host, nginxProxyEnv, nginxProxyImage string) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: nginxProxyImage,
		Env:   []string{fmt.Sprintf("%s=%s", NginxProxyEnvName, nginxProxyEnv)},
	}
	hostCfg := &container.HostConfig{
		NetworkMode:   "host",
		RestartPolicy: container.RestartPolicy{Name: "always"},
	}

	return imageCfg, hostCfg
}

func buildProxyEnv(cpHosts []*hosts.Host) string {
	proxyEnv := ""
	for i, cpHost := range cpHosts {
		proxyEnv += fmt.Sprintf("%s", cpHost.InternalAddress)
		if i < (len(cpHosts) - 1) {
			proxyEnv += ","
		}
	}
	return proxyEnv
}
