package pki

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	StateDeployerContainerName = "cluster-state-deployer"
)

func DeployCertificatesOnPlaneHost(ctx context.Context, host *hosts.Host, rkeConfig v3.RancherKubernetesEngineConfig, crtMap map[string]CertificatePKI, certDownloaderImage string, prsMap map[string]v3.PrivateRegistry, forceDeploy bool) error {
	crtBundle := GenerateRKENodeCerts(ctx, rkeConfig, host.Address, crtMap)
	env := []string{}
	for _, crt := range crtBundle {
		env = append(env, crt.ToEnv()...)
	}
	if forceDeploy {
		env = append(env, "FORCE_DEPLOY=true")
	}
	return doRunDeployer(ctx, host, env, certDownloaderImage, prsMap)
}

func DeployStateOnPlaneHost(ctx context.Context, host *hosts.Host, stateDownloaderImage string, prsMap map[string]v3.PrivateRegistry, clusterState string) error {
	// remove existing container. Only way it's still here is if previous deployment failed
	if err := docker.DoRemoveContainer(ctx, host.DClient, StateDeployerContainerName, host.Address); err != nil {
		return err
	}
	containerEnv := []string{ClusterStateEnv + "=" + clusterState}
	ClusterStateFilePath := path.Join(host.PrefixPath, TempCertPath, ClusterStateFile)
	imageCfg := &container.Config{
		Image: stateDownloaderImage,
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf("t=$(mktemp); echo -e \"$%s\" > $t && mv $t %s && chmod 644 %s", ClusterStateEnv, ClusterStateFilePath, ClusterStateFilePath),
		},
		Env: containerEnv,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
		},
		Privileged: true,
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, StateDeployerContainerName, host.Address, "state", prsMap); err != nil {
		return err
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, StateDeployerContainerName, host.Address); err != nil {
		return err
	}
	logrus.Debugf("[state] Successfully started state deployer container on node [%s]", host.Address)
	return nil
}

func doRunDeployer(ctx context.Context, host *hosts.Host, containerEnv []string, certDownloaderImage string, prsMap map[string]v3.PrivateRegistry) error {
	// remove existing container. Only way it's still here is if previous deployment failed
	isRunning := false
	isRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, CrtDownloaderContainer, true)
	if err != nil {
		return err
	}
	if isRunning {
		if err := docker.RemoveContainer(ctx, host.DClient, host.Address, CrtDownloaderContainer); err != nil {
			return err
		}
	}
	if err := docker.UseLocalOrPull(ctx, host.DClient, host.Address, certDownloaderImage, CertificatesServiceName, prsMap); err != nil {
		return err
	}
	imageCfg := &container.Config{
		Image: certDownloaderImage,
		Cmd:   []string{"cert-deployer"},
		Env:   containerEnv,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
		},
		Privileged: true,
	}
	resp, err := host.DClient.ContainerCreate(ctx, imageCfg, hostCfg, nil, CrtDownloaderContainer)
	if err != nil {
		return fmt.Errorf("Failed to create Certificates deployer container on host [%s]: %v", host.Address, err)
	}

	if err := host.DClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Failed to start Certificates deployer container on host [%s]: %v", host.Address, err)
	}
	logrus.Debugf("[certificates] Successfully started Certificate deployer container: %s", resp.ID)
	for {
		isDeployerRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, CrtDownloaderContainer, false)
		if err != nil {
			return err
		}
		if isDeployerRunning {
			time.Sleep(5 * time.Second)
			continue
		}
		if err := host.DClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{RemoveVolumes: true}); err != nil {
			return fmt.Errorf("Failed to delete Certificates deployer container on host [%s]: %v", host.Address, err)
		}
		return nil
	}
}

func DeployAdminConfig(ctx context.Context, kubeConfig, localConfigPath string) error {
	if len(kubeConfig) == 0 {
		return nil
	}
	logrus.Debugf("Deploying admin Kubeconfig locally: %s", kubeConfig)
	err := ioutil.WriteFile(localConfigPath, []byte(kubeConfig), 0640)
	if err != nil {
		return fmt.Errorf("Failed to create local admin kubeconfig file: %v", err)
	}
	log.Infof(ctx, "Successfully Deployed local admin kubeconfig at [%s]", localConfigPath)
	return nil
}

func RemoveAdminConfig(ctx context.Context, localConfigPath string) {
	log.Infof(ctx, "Removing local admin Kubeconfig: %s", localConfigPath)
	if err := os.Remove(localConfigPath); err != nil {
		logrus.Warningf("Failed to remove local admin Kubeconfig file: %v", err)
		return
	}
	log.Infof(ctx, "Local admin Kubeconfig removed successfully")
}

func DeployCertificatesOnHost(ctx context.Context, host *hosts.Host, crtMap map[string]CertificatePKI, certDownloaderImage, certPath string, prsMap map[string]v3.PrivateRegistry) error {
	env := []string{
		"CRTS_DEPLOY_PATH=" + certPath,
	}
	for _, crt := range crtMap {
		env = append(env, crt.ToEnv()...)
	}
	return doRunDeployer(ctx, host, env, certDownloaderImage, prsMap)
}
