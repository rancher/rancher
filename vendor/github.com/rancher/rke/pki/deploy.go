package pki

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki/cert"
	v3 "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
)

const (
	StateDeployerContainerName = "cluster-state-deployer"
)

func DeployCertificatesOnPlaneHost(
	ctx context.Context,
	host *hosts.Host,
	rkeConfig v3.RancherKubernetesEngineConfig,
	crtMap map[string]CertificatePKI,
	certDownloaderImage string,
	prsMap map[string]v3.PrivateRegistry,
	forceDeploy bool,
	env []string) error {
	crtBundle := GenerateRKENodeCerts(ctx, rkeConfig, host.Address, crtMap)

	// Strip CA key as its sensitive and unneeded on nodes without controlplane role
	if !host.IsControl {
		caCert := crtBundle[CACertName]
		caCert.Key = nil
		caCert.KeyEnvName = ""
		caCert.KeyPath = ""
		crtBundle[CACertName] = caCert
	}

	for _, crt := range crtBundle {
		env = append(env, crt.ToEnv()...)
	}
	if forceDeploy {
		env = append(env, "FORCE_DEPLOY=true")
	}
	if host.IsEtcd &&
		rkeConfig.Services.Etcd.UID != 0 &&
		rkeConfig.Services.Etcd.GID != 0 {
		env = append(env,
			[]string{fmt.Sprintf("ETCD_UID=%d", rkeConfig.Services.Etcd.UID),
				fmt.Sprintf("ETCD_GID=%d", rkeConfig.Services.Etcd.GID)}...)
	}

	return doRunDeployer(ctx, host, env, certDownloaderImage, prsMap)
}

func DeployStateOnPlaneHost(ctx context.Context, host *hosts.Host, stateDownloaderImage string, prsMap map[string]v3.PrivateRegistry, stateFilePath, snapshotName string) error {
	// remove existing container. Only way it's still here is if previous deployment failed
	if err := docker.DoRemoveContainer(ctx, host.DClient, StateDeployerContainerName, host.Address); err != nil {
		return err
	}
	// This is the location it needs to end up for rke-tools to pick it up and include it in the snapshot
	// Example: /etc/kubernetes/snapshotname.rkestate
	DestinationClusterStateFilePath := path.Join(K8sBaseDir, "/", fmt.Sprintf("%s%s", snapshotName, ClusterStateExt))
	// This is the location where the 1-on-1 copy from local will be placed in the container, this is later moved to DestinationClusterStateFilePath
	// Example: /etc/kubernetes/cluster.rkestate
	// Example: /etc/kubernetes/rancher-cluster.rkestate
	baseStateFile := path.Base(stateFilePath)
	SourceClusterStateFilePath := path.Join(K8sBaseDir, baseStateFile)
	logrus.Infof("[state] Deploying state file to [%v] on host [%s]", DestinationClusterStateFilePath, host.Address)

	imageCfg := &container.Config{
		Image: stateDownloaderImage,
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf("for i in $(seq 1 12); do if [ -f \"%[1]s\" ]; then echo \"File [%[1]s] present in this container\"; echo \"Moving [%[1]s] to [%[2]s]\"; mv %[1]s %[2]s; echo \"State file successfully moved to [%[2]s]\"; echo \"Changing permissions to 0400\"; chmod 400 %[2]s; break; else echo \"Waiting for file [%[1]s] to be successfully copied to this container, retry count $i\"; sleep 5; fi; done", SourceClusterStateFilePath, DestinationClusterStateFilePath),
		},
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
	tarFile, err := archive.Tar(stateFilePath, archive.Uncompressed)
	if err != nil {
		// Snapshot is still valid without containing the state file
		logrus.Warnf("[state] Error during creating archive tar to copy for local cluster state file [%s] on host [%s]: %v", stateFilePath, host.Address, err)
	}
	if err := docker.DoCopyToContainer(ctx, host.DClient, "state", StateDeployerContainerName, host.Address, K8sBaseDir, tarFile); err != nil {
		// Snapshot is still valid without containing the state file
		logrus.Warnf("[state] Error during copying state file [%s] to node [%s]: %v", stateFilePath, host.Address, err)
	}

	if _, err := docker.WaitForContainer(ctx, host.DClient, host.Address, StateDeployerContainerName); err != nil {
		return err
	}

	if err := docker.DoRemoveContainer(ctx, host.DClient, StateDeployerContainerName, host.Address); err != nil {
		return err
	}
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
	if host.IsWindows() { // compatible with Windows
		imageCfg = &container.Config{
			Image: certDownloaderImage,
			Cmd: []string{
				"pwsh", "-NoLogo", "-NonInteractive", "-File", "c:/usr/bin/cert-deployer.ps1",
			},
			Env: containerEnv,
		}
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
		},
		Privileged: true,
	}
	if host.IsWindows() { // compatible with Windows
		hostCfg = &container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s:c:/etc/kubernetes", path.Join(host.PrefixPath, "/etc/kubernetes")),
			},
		}
	}
	_, err = docker.CreateContainer(ctx, host.DClient, host.Address, CrtDownloaderContainer, imageCfg, hostCfg)
	if err != nil {
		return fmt.Errorf("Failed to create Certificates deployer container on host [%s]: %v", host.Address, err)
	}

	if err := docker.StartContainer(ctx, host.DClient, host.Address, CrtDownloaderContainer); err != nil {
		return fmt.Errorf("Failed to start Certificates deployer container on host [%s]: %v", host.Address, err)
	}
	logrus.Debugf("[certificates] Successfully started Certificate deployer container: %s", CrtDownloaderContainer)
	for {
		isDeployerRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, CrtDownloaderContainer, false)
		if err != nil {
			return err
		}
		if isDeployerRunning {
			time.Sleep(5 * time.Second)
			continue
		}
		if err := docker.RemoveContainer(ctx, host.DClient, host.Address, CrtDownloaderContainer); err != nil {
			return fmt.Errorf("Failed to delete Certificates deployer container on host [%s]: %v", host.Address, err)
		}
		return nil
	}
}

func DeployAdminConfig(ctx context.Context, kubeConfig, localConfigPath string) error {
	if len(kubeConfig) == 0 {
		return nil
	}
	logrus.Debugf("Deploying admin Kubeconfig locally at [%s]", localConfigPath)
	logrus.Tracef("Deploying admin Kubeconfig locally: %s", kubeConfig)
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

func FetchCertificatesFromHost(ctx context.Context, extraHosts []*hosts.Host, host *hosts.Host, image, localConfigPath string, prsMap map[string]v3.PrivateRegistry) (map[string]CertificatePKI, error) {
	// rebuilding the certificates. This should look better after refactoring pki
	tmpCerts := make(map[string]CertificatePKI)

	crtList := map[string]bool{
		CACertName:              false,
		KubeAPICertName:         false,
		KubeControllerCertName:  true,
		KubeSchedulerCertName:   true,
		KubeProxyCertName:       true,
		KubeNodeCertName:        true,
		KubeAdminCertName:       false,
		RequestHeaderCACertName: false,
		APIProxyClientCertName:  false,
	}

	for _, etcdHost := range extraHosts {
		// Fetch etcd certificates
		crtList[GetCrtNameForHost(etcdHost, EtcdCertName)] = false
	}

	for certName, config := range crtList {
		certificate := CertificatePKI{}
		crt, err := FetchFileFromHost(ctx, GetCertTempPath(certName), image, host, prsMap, CertFetcherContainer, "certificates")
		// Return error if the certificate file is not found but only if its not etcd or request header certificate
		if err != nil && !strings.HasPrefix(certName, "kube-etcd") &&
			certName != RequestHeaderCACertName &&
			certName != APIProxyClientCertName &&
			certName != KubeAdminCertName {
			// IsErrNotFound doesn't catch this because it's a custom error
			if isFileNotFoundErr(err) {
				return nil, fmt.Errorf("Certificate %s is not found", GetCertTempPath(certName))
			}
			return nil, err

		}
		// If I can't find an etcd or request header ca I will not fail and will create it later.
		if crt == "" && (strings.HasPrefix(certName, "kube-etcd") ||
			certName == RequestHeaderCACertName ||
			certName == APIProxyClientCertName ||
			certName == KubeAdminCertName) {
			tmpCerts[certName] = CertificatePKI{}
			continue
		}
		key, err := FetchFileFromHost(ctx, GetKeyTempPath(certName), image, host, prsMap, CertFetcherContainer, "certificate")
		if err != nil {
			if isFileNotFoundErr(err) {
				return nil, fmt.Errorf("Key %s is not found", GetKeyTempPath(certName))
			}
			return nil, err
		}
		if config {
			config, err := FetchFileFromHost(ctx, GetConfigTempPath(certName), image, host, prsMap, CertFetcherContainer, "certificate")
			if err != nil {
				if isFileNotFoundErr(err) {
					return nil, fmt.Errorf("Config %s is not found", GetConfigTempPath(certName))
				}
				return nil, err
			}
			certificate.Config = config
		}
		parsedCert, err := cert.ParseCertsPEM([]byte(crt))
		if err != nil {
			return nil, err
		}
		parsedKey, err := cert.ParsePrivateKeyPEM([]byte(key))
		if err != nil {
			return nil, err
		}
		certificate.Certificate = parsedCert[0]
		certificate.Key = parsedKey.(*rsa.PrivateKey)
		tmpCerts[certName] = certificate
		logrus.Debugf("[certificates] Recovered certificate: %s", certName)
	}

	if err := docker.RemoveContainer(ctx, host.DClient, host.Address, CertFetcherContainer); err != nil {
		return nil, err
	}
	return populateCertMap(tmpCerts, localConfigPath, extraHosts), nil

}

func FetchFileFromHost(ctx context.Context, filePath, image string, host *hosts.Host, prsMap map[string]v3.PrivateRegistry, containerName, state string) (string, error) {
	imageCfg := &container.Config{
		Image: image,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
		},
		Privileged: true,
	}
	isRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, containerName, true)
	if err != nil {
		return "", err
	}
	if !isRunning {
		if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, containerName, host.Address, state, prsMap); err != nil {
			return "", err
		}
	}
	file, err := docker.ReadFileFromContainer(ctx, host.DClient, host.Address, containerName, filePath)
	if err != nil {
		return "", err
	}

	return file, nil
}
