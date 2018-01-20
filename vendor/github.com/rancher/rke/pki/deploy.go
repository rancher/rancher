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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/cert"
)

func DeployCertificatesOnMasters(ctx context.Context, cpHosts []*hosts.Host, crtMap map[string]CertificatePKI, certDownloaderImage string) error {
	// list of certificates that should be deployed on the masters
	crtList := []string{
		CACertName,
		KubeAPICertName,
		KubeControllerName,
		KubeSchedulerName,
		KubeProxyName,
		KubeNodeName,
	}
	env := []string{}
	for _, crtName := range crtList {
		c := crtMap[crtName]
		env = append(env, c.ToEnv()...)
	}

	for i := range cpHosts {
		err := doRunDeployer(ctx, cpHosts[i], env, certDownloaderImage)
		if err != nil {
			return err
		}
	}
	return nil
}

func DeployCertificatesOnWorkers(ctx context.Context, workerHosts []*hosts.Host, crtMap map[string]CertificatePKI, certDownloaderImage string) error {
	// list of certificates that should be deployed on the workers
	crtList := []string{
		CACertName,
		KubeProxyName,
		KubeNodeName,
	}
	env := []string{}
	for _, crtName := range crtList {
		c := crtMap[crtName]
		env = append(env, c.ToEnv()...)
	}

	for i := range workerHosts {
		err := doRunDeployer(ctx, workerHosts[i], env, certDownloaderImage)
		if err != nil {
			return err
		}
	}
	return nil
}

func doRunDeployer(ctx context.Context, host *hosts.Host, containerEnv []string, certDownloaderImage string) error {
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
	if err := docker.UseLocalOrPull(ctx, host.DClient, host.Address, certDownloaderImage, CertificatesServiceName); err != nil {
		return err
	}
	imageCfg := &container.Config{
		Image: certDownloaderImage,
		Env:   containerEnv,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
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
		if err := host.DClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
			return fmt.Errorf("Failed to delete Certificates deployer container on host [%s]: %v", host.Address, err)
		}
		return nil
	}
}

func DeployAdminConfig(ctx context.Context, kubeConfig, localConfigPath string) error {
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

func DeployCertificatesOnHost(ctx context.Context, host *hosts.Host, crtMap map[string]CertificatePKI, certDownloaderImage string) error {
	crtList := []string{
		CACertName,
		KubeAPICertName,
		KubeControllerName,
		KubeSchedulerName,
		KubeProxyName,
		KubeNodeName,
		KubeAdminCommonName,
	}

	env := []string{
		"CRTS_DEPLOY_PATH=" + TempCertPath,
	}
	for _, crtName := range crtList {
		c := crtMap[crtName]
		// We don't need to edit the cert paths, they are not used in deployment
		env = append(env, c.ToEnv()...)
	}
	return doRunDeployer(ctx, host, env, certDownloaderImage)
}

func FetchCertificatesFromHost(ctx context.Context, host *hosts.Host, image, localConfigPath string) (map[string]CertificatePKI, error) {
	// rebuilding the certificates. This should look better after refactoring pki
	tmpCerts := make(map[string]CertificatePKI)

	certEnvMap := map[string][]string{
		CACertName:          []string{CACertPath, CAKeyPath},
		KubeAPICertName:     []string{KubeAPICertPath, KubeAPIKeyPath},
		KubeControllerName:  []string{KubeControllerCertPath, KubeControllerKeyPath, KubeControllerConfigPath},
		KubeSchedulerName:   []string{KubeSchedulerCertPath, KubeSchedulerKeyPath, KubeSchedulerConfigPath},
		KubeProxyName:       []string{KubeProxyCertPath, KubeProxyKeyPath, KubeProxyConfigPath},
		KubeNodeName:        []string{KubeNodeCertPath, KubeNodeKeyPath, KubeNodeConfigPath},
		KubeAdminCommonName: []string{"kube-admin.pem", "kube-admin-key.pem", "kubecfg-admin.yaml"},
	}
	// get files from hosts

	for crtName, certEnv := range certEnvMap {
		certificate := CertificatePKI{}
		crt, err := fetchFileFromHost(ctx, getTempPath(certEnv[0]), image, host)
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") ||
				strings.Contains(err.Error(), "Could not find the file") {
				return nil, nil
			}
			return nil, err
		}
		key, err := fetchFileFromHost(ctx, getTempPath(certEnv[1]), image, host)

		if len(certEnv) > 2 {
			config, err := fetchFileFromHost(ctx, getTempPath(certEnv[2]), image, host)
			if err != nil {
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
		tmpCerts[crtName] = certificate
		logrus.Debugf("[certificates] Recovered certificate: %s", crtName)
	}

	if err := docker.RemoveContainer(ctx, host.DClient, host.Address, CertFetcherContainer); err != nil {
		return nil, err
	}
	return populateCertMap(tmpCerts, localConfigPath), nil

}

func fetchFileFromHost(ctx context.Context, filePath, image string, host *hosts.Host) (string, error) {

	imageCfg := &container.Config{
		Image: image,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
		},
		Privileged: true,
	}
	isRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, CertFetcherContainer, true)
	if err != nil {
		return "", err
	}
	if !isRunning {
		if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, CertFetcherContainer, host.Address, "certificates"); err != nil {
			return "", err
		}
	}
	file, err := docker.ReadFileFromContainer(ctx, host.DClient, host.Address, CertFetcherContainer, filePath)
	if err != nil {
		return "", err
	}

	return file, nil
}

func getTempPath(s string) string {
	return TempCertPath + path.Base(s)
}

func populateCertMap(tmpCerts map[string]CertificatePKI, localConfigPath string) map[string]CertificatePKI {
	certs := make(map[string]CertificatePKI)
	//CACert
	certs[CACertName] = CertificatePKI{
		Certificate: tmpCerts[CACertName].Certificate,
		Key:         tmpCerts[CACertName].Key,
		Name:        CACertName,
		EnvName:     CACertENVName,
		KeyEnvName:  CAKeyENVName,
		Path:        CACertPath,
		KeyPath:     CAKeyPath,
	}
	//KubeAPI
	certs[KubeAPICertName] = CertificatePKI{
		Certificate: tmpCerts[KubeAPICertName].Certificate,
		Key:         tmpCerts[KubeAPICertName].Key,
		Name:        KubeAPICertName,
		EnvName:     KubeAPICertENVName,
		KeyEnvName:  KubeAPIKeyENVName,
		Path:        KubeAPICertPath,
		KeyPath:     KubeAPIKeyPath,
	}
	//kubeController
	certs[KubeControllerName] = CertificatePKI{
		Certificate:   tmpCerts[KubeControllerName].Certificate,
		Key:           tmpCerts[KubeControllerName].Key,
		Config:        tmpCerts[KubeControllerName].Config,
		Name:          KubeControllerName,
		CommonName:    KubeControllerCommonName,
		EnvName:       KubeControllerCertENVName,
		KeyEnvName:    KubeControllerKeyENVName,
		Path:          KubeControllerCertPath,
		KeyPath:       KubeControllerKeyPath,
		ConfigEnvName: KubeControllerConfigENVName,
		ConfigPath:    KubeControllerConfigPath,
	}
	//KubeScheduler
	certs[KubeSchedulerName] = CertificatePKI{
		Certificate:   tmpCerts[KubeSchedulerName].Certificate,
		Key:           tmpCerts[KubeSchedulerName].Key,
		Config:        tmpCerts[KubeSchedulerName].Config,
		Name:          KubeSchedulerName,
		CommonName:    KubeSchedulerCommonName,
		EnvName:       KubeSchedulerCertENVName,
		KeyEnvName:    KubeSchedulerKeyENVName,
		Path:          KubeSchedulerCertPath,
		KeyPath:       KubeSchedulerKeyPath,
		ConfigEnvName: KubeSchedulerConfigENVName,
		ConfigPath:    KubeSchedulerConfigPath,
	}
	// KubeProxy
	certs[KubeProxyName] = CertificatePKI{
		Certificate:   tmpCerts[KubeProxyName].Certificate,
		Key:           tmpCerts[KubeProxyName].Key,
		Config:        tmpCerts[KubeProxyName].Config,
		Name:          KubeProxyName,
		CommonName:    KubeProxyCommonName,
		EnvName:       KubeProxyCertENVName,
		Path:          KubeProxyCertPath,
		KeyEnvName:    KubeProxyKeyENVName,
		KeyPath:       KubeProxyKeyPath,
		ConfigEnvName: KubeProxyConfigENVName,
		ConfigPath:    KubeProxyConfigPath,
	}
	// KubeNode
	certs[KubeNodeName] = CertificatePKI{
		Certificate:   tmpCerts[KubeNodeName].Certificate,
		Key:           tmpCerts[KubeNodeName].Key,
		Config:        tmpCerts[KubeNodeName].Config,
		Name:          KubeNodeName,
		CommonName:    KubeNodeCommonName,
		OUName:        KubeNodeOrganizationName,
		EnvName:       KubeNodeCertENVName,
		KeyEnvName:    KubeNodeKeyENVName,
		Path:          KubeNodeCertPath,
		KeyPath:       KubeNodeKeyPath,
		ConfigEnvName: KubeNodeConfigENVName,
		ConfigPath:    KubeNodeCommonName,
	}

	certs[KubeAdminCommonName] = CertificatePKI{
		Certificate:   tmpCerts[KubeAdminCommonName].Certificate,
		Key:           tmpCerts[KubeAdminCommonName].Key,
		Config:        tmpCerts[KubeAdminCommonName].Config,
		CommonName:    KubeAdminCommonName,
		OUName:        KubeAdminOrganizationName,
		ConfigEnvName: KubeAdminConfigENVName,
		ConfigPath:    localConfigPath,
		EnvName:       KubeAdminCertEnvName,
		KeyEnvName:    KubeAdminKeyEnvName,
	}
	return certs
}
