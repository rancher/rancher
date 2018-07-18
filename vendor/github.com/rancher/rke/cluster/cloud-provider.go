package cluster

import (
	"context"
	"fmt"
	"path"

	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	CloudConfigDeployer    = "cloud-config-deployer"
	CloudConfigServiceName = "cloud"
	CloudConfigPath        = "/etc/kubernetes/cloud-config"
	CloudConfigEnv         = "RKE_CLOUD_CONFIG"
)

func deployCloudProviderConfig(ctx context.Context, uniqueHosts []*hosts.Host, alpineImage string, prsMap map[string]v3.PrivateRegistry, cloudConfig string) error {
	for _, host := range uniqueHosts {
		log.Infof(ctx, "[%s] Deploying cloud config file to node [%s]", CloudConfigServiceName, host.Address)
		if err := doDeployConfigFile(ctx, host, cloudConfig, alpineImage, prsMap); err != nil {
			return fmt.Errorf("Failed to deploy cloud config file on node [%s]: %v", host.Address, err)
		}
	}
	return nil
}

func doDeployConfigFile(ctx context.Context, host *hosts.Host, cloudConfig, alpineImage string, prsMap map[string]v3.PrivateRegistry) error {
	// remove existing container. Only way it's still here is if previous deployment failed
	if err := docker.DoRemoveContainer(ctx, host.DClient, CloudConfigDeployer, host.Address); err != nil {
		return err
	}
	containerEnv := []string{CloudConfigEnv + "=" + cloudConfig}
	imageCfg := &container.Config{
		Image: alpineImage,
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf("t=$(mktemp); echo -e \"$%s\" > $t && mv $t %s && chmod 644 %s", CloudConfigEnv, CloudConfigPath, CloudConfigPath),
		},
		Env: containerEnv,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
		},
		Privileged: true,
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, CloudConfigDeployer, host.Address, CloudConfigServiceName, prsMap); err != nil {
		return err
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, CloudConfigDeployer, host.Address); err != nil {
		return err
	}
	logrus.Debugf("[%s] Successfully started cloud config deployer container on node [%s]", CloudConfigServiceName, host.Address)
	return nil
}
