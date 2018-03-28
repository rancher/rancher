package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

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
	CloudConfigPath        = "/etc/kubernetes/cloud-config.json"
	CloudConfigEnv         = "RKE_CLOUD_CONFIG"
)

func deployCloudProviderConfig(ctx context.Context, uniqueHosts []*hosts.Host, cloudProvider v3.CloudProvider, alpineImage string, prsMap map[string]v3.PrivateRegistry) error {
	cloudConfig, err := getCloudConfigFile(ctx, cloudProvider)
	if err != nil {
		return err
	}
	for _, host := range uniqueHosts {
		log.Infof(ctx, "[%s] Deploying cloud config file to node [%s]", CloudConfigServiceName, host.Address)
		if err := doDeployConfigFile(ctx, host, cloudConfig, alpineImage, prsMap); err != nil {
			return fmt.Errorf("Failed to deploy cloud config file on node [%s]: %v", host.Address, err)
		}
	}
	return nil
}

func getCloudConfigFile(ctx context.Context, cloudProvider v3.CloudProvider) (string, error) {
	if len(cloudProvider.CloudConfig) == 0 {
		return "", nil
	}
	tmpMap := make(map[string]interface{})
	for key, value := range cloudProvider.CloudConfig {
		tmpBool, err := strconv.ParseBool(value)
		if err == nil {
			tmpMap[key] = tmpBool
			continue
		}
		tmpInt, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			tmpMap[key] = tmpInt
			continue
		}
		tmpFloat, err := strconv.ParseFloat(value, 64)
		if err == nil {
			tmpMap[key] = tmpFloat
			continue
		}
		tmpMap[key] = value
	}
	jsonString, err := json.MarshalIndent(tmpMap, "", "\n")
	if err != nil {
		return "", err
	}
	return string(jsonString), nil
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
			fmt.Sprintf("if [ ! -f %s ]; then echo -e \"$%s\" > %s;fi", CloudConfigPath, CloudConfigEnv, CloudConfigPath),
		},
		Env: containerEnv,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
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
