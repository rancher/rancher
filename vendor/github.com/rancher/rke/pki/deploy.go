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
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/cert"
)

func DeployCertificatesOnPlaneHost(ctx context.Context, host *hosts.Host, rkeConfig v3.RancherKubernetesEngineConfig, crtMap map[string]CertificatePKI, certDownloaderImage string, prsMap map[string]v3.PrivateRegistry) error {
	crtBundle := GenerateRKENodeCerts(ctx, rkeConfig, host.Address, crtMap)
	env := []string{}
	for _, crt := range crtBundle {
		env = append(env, crt.ToEnv()...)
	}
	return doRunDeployer(ctx, host, env, certDownloaderImage, prsMap)
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
		if err := host.DClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
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

func FetchCertificatesFromHost(ctx context.Context, extraHosts []*hosts.Host, host *hosts.Host, image, localConfigPath string, prsMap map[string]v3.PrivateRegistry) (map[string]CertificatePKI, error) {
	// rebuilding the certificates. This should look better after refactoring pki
	tmpCerts := make(map[string]CertificatePKI)

	crtList := map[string]bool{
		CACertName:             false,
		KubeAPICertName:        false,
		KubeControllerCertName: true,
		KubeSchedulerCertName:  true,
		KubeProxyCertName:      true,
		KubeNodeCertName:       true,
		KubeAdminCertName:      false,
	}

	for _, etcdHost := range extraHosts {
		// Fetch etcd certificates
		crtList[GetEtcdCrtName(etcdHost.InternalAddress)] = false
	}

	for certName, config := range crtList {
		certificate := CertificatePKI{}
		crt, err := FetchFileFromHost(ctx, GetCertTempPath(certName), image, host, prsMap)
		// I will only exit with an error if it's not a not-found-error and this is not an etcd certificate
		if err != nil && !strings.HasPrefix(certName, "kube-etcd") {
			if strings.Contains(err.Error(), "no such file or directory") ||
				strings.Contains(err.Error(), "Could not find the file") {
				return nil, nil
			}
			return nil, err

		}
		// If I can't find an etcd cert, I will not fail and will create it later.
		if crt == "" && strings.HasPrefix(certName, "kube-etcd") {
			tmpCerts[certName] = CertificatePKI{}
			continue
		}
		key, err := FetchFileFromHost(ctx, GetKeyTempPath(certName), image, host, prsMap)

		if config {
			config, err := FetchFileFromHost(ctx, GetConfigTempPath(certName), image, host, prsMap)
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
		tmpCerts[certName] = certificate
		logrus.Debugf("[certificates] Recovered certificate: %s", certName)
	}

	if err := docker.RemoveContainer(ctx, host.DClient, host.Address, CertFetcherContainer); err != nil {
		return nil, err
	}
	return populateCertMap(tmpCerts, localConfigPath, extraHosts), nil

}

func FetchFileFromHost(ctx context.Context, filePath, image string, host *hosts.Host, prsMap map[string]v3.PrivateRegistry) (string, error) {

	imageCfg := &container.Config{
		Image: image,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
		},
		Privileged: true,
	}
	isRunning, err := docker.IsContainerRunning(ctx, host.DClient, host.Address, CertFetcherContainer, true)
	if err != nil {
		return "", err
	}
	if !isRunning {
		if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, CertFetcherContainer, host.Address, "certificates", prsMap); err != nil {
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

func populateCertMap(tmpCerts map[string]CertificatePKI, localConfigPath string, extraHosts []*hosts.Host) map[string]CertificatePKI {
	certs := make(map[string]CertificatePKI)
	// CACert
	certs[CACertName] = ToCertObject(CACertName, "", "", tmpCerts[CACertName].Certificate, tmpCerts[CACertName].Key)
	// KubeAPI
	certs[KubeAPICertName] = ToCertObject(KubeAPICertName, "", "", tmpCerts[KubeAPICertName].Certificate, tmpCerts[KubeAPICertName].Key)
	// kubeController
	certs[KubeControllerCertName] = ToCertObject(KubeControllerCertName, "", "", tmpCerts[KubeControllerCertName].Certificate, tmpCerts[KubeControllerCertName].Key)
	// KubeScheduler
	certs[KubeSchedulerCertName] = ToCertObject(KubeSchedulerCertName, "", "", tmpCerts[KubeSchedulerCertName].Certificate, tmpCerts[KubeSchedulerCertName].Key)
	// KubeProxy
	certs[KubeProxyCertName] = ToCertObject(KubeProxyCertName, "", "", tmpCerts[KubeProxyCertName].Certificate, tmpCerts[KubeProxyCertName].Key)
	// KubeNode
	certs[KubeNodeCertName] = ToCertObject(KubeNodeCertName, KubeNodeCommonName, KubeNodeOrganizationName, tmpCerts[KubeNodeCertName].Certificate, tmpCerts[KubeNodeCertName].Key)
	// KubeAdmin
	kubeAdminCertObj := ToCertObject(KubeAdminCertName, KubeAdminCertName, KubeAdminOrganizationName, tmpCerts[KubeAdminCertName].Certificate, tmpCerts[KubeAdminCertName].Key)
	kubeAdminCertObj.Config = tmpCerts[KubeAdminCertName].Config
	kubeAdminCertObj.ConfigPath = localConfigPath
	certs[KubeAdminCertName] = kubeAdminCertObj
	// etcd
	for _, host := range extraHosts {
		etcdName := GetEtcdCrtName(host.InternalAddress)
		etcdCrt, etcdKey := tmpCerts[etcdName].Certificate, tmpCerts[etcdName].Key
		certs[etcdName] = ToCertObject(etcdName, "", "", etcdCrt, etcdKey)
	}

	return certs
}
