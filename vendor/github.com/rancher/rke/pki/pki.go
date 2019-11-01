package pki

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type CertificatePKI struct {
	Certificate    *x509.Certificate        `json:"-"`
	Key            *rsa.PrivateKey          `json:"-"`
	CSR            *x509.CertificateRequest `json:"-"`
	CertificatePEM string                   `json:"certificatePEM"`
	KeyPEM         string                   `json:"keyPEM"`
	CSRPEM         string                   `json:"-"`
	Config         string                   `json:"config"`
	Name           string                   `json:"name"`
	CommonName     string                   `json:"commonName"`
	OUName         string                   `json:"ouName"`
	EnvName        string                   `json:"envName"`
	Path           string                   `json:"path"`
	KeyEnvName     string                   `json:"keyEnvName"`
	KeyPath        string                   `json:"keyPath"`
	ConfigEnvName  string                   `json:"configEnvName"`
	ConfigPath     string                   `json:"configPath"`
}

type GenFunc func(context.Context, map[string]CertificatePKI, v3.RancherKubernetesEngineConfig, string, string, bool) error
type CSRFunc func(context.Context, map[string]CertificatePKI, v3.RancherKubernetesEngineConfig) error

const (
	etcdRole            = "etcd"
	controlRole         = "controlplane"
	workerRole          = "worker"
	BundleCertContainer = "rke-bundle-cert"
)

func GenerateRKECerts(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, configPath, configDir string) (map[string]CertificatePKI, error) {
	certs := make(map[string]CertificatePKI)
	// generate RKE CA certificates
	if err := GenerateRKECACerts(ctx, certs, configPath, configDir); err != nil {
		return certs, err
	}
	// Generating certificates for kubernetes components
	if err := GenerateRKEServicesCerts(ctx, certs, rkeConfig, configPath, configDir, false); err != nil {
		return certs, err
	}
	return certs, nil
}

func GenerateRKENodeCerts(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, nodeAddress string, certBundle map[string]CertificatePKI) map[string]CertificatePKI {
	crtMap := make(map[string]CertificatePKI)
	crtKeys := []string{}
	for _, node := range rkeConfig.Nodes {
		if node.Address == nodeAddress {
			for _, role := range node.Role {
				keys := getCertKeys(rkeConfig.Nodes, role, &rkeConfig)
				crtKeys = append(crtKeys, keys...)
			}
			break
		}
	}
	for _, key := range crtKeys {
		crtMap[key] = certBundle[key]
	}
	return crtMap
}

func RegenerateEtcdCertificate(
	ctx context.Context,
	crtMap map[string]CertificatePKI,
	etcdHost *hosts.Host,
	etcdHosts []*hosts.Host,
	clusterDomain string,
	KubernetesServiceIP net.IP) (map[string]CertificatePKI, error) {

	etcdName := GetCrtNameForHost(etcdHost, EtcdCertName)
	log.Infof(ctx, "[certificates] Regenerating new %s certificate and key", etcdName)
	caCrt := crtMap[CACertName].Certificate
	caKey := crtMap[CACertName].Key
	etcdAltNames := GetAltNames(etcdHosts, clusterDomain, KubernetesServiceIP, []string{})

	etcdCrt, etcdKey, err := GenerateSignedCertAndKey(caCrt, caKey, true, EtcdCertName, etcdAltNames, nil, nil)
	if err != nil {
		return nil, err
	}
	crtMap[etcdName] = ToCertObject(etcdName, "", "", etcdCrt, etcdKey, nil)
	log.Infof(ctx, "[certificates] Successfully generated new %s certificate and key", etcdName)
	return crtMap, nil
}

func SaveBackupBundleOnHost(ctx context.Context, host *hosts.Host, alpineSystemImage, etcdSnapshotPath string, prsMap map[string]v3.PrivateRegistry) error {
	imageCfg := &container.Config{
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf("if [ -d %s ] && [ \"$(ls -A %s)\" ]; then tar czvf %s %s;fi", TempCertPath, TempCertPath, BundleCertPath, TempCertPath),
		},
		Image: alpineSystemImage,
	}
	hostCfg := &container.HostConfig{

		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
			fmt.Sprintf("%s:/backup:z", etcdSnapshotPath),
		},
		Privileged: true,
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, BundleCertContainer, host.Address, "certificates", prsMap); err != nil {
		return err
	}
	status, err := docker.WaitForContainer(ctx, host.DClient, host.Address, BundleCertContainer)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("Failed to run certificate bundle compress, exit status is: %d", status)
	}
	log.Infof(ctx, "[certificates] successfully saved certificate bundle [%s/pki.bundle.tar.gz] on host [%s]", etcdSnapshotPath, host.Address)
	return docker.RemoveContainer(ctx, host.DClient, host.Address, BundleCertContainer)
}

func ExtractBackupBundleOnHost(ctx context.Context, host *hosts.Host, alpineSystemImage, etcdSnapshotPath string, prsMap map[string]v3.PrivateRegistry) error {
	imageCfg := &container.Config{
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf(
				"mkdir -p %s; tar xzvf %s -C %s --strip-components %d --exclude %s",
				TempCertPath,
				BundleCertPath,
				TempCertPath,
				len(strings.Split(filepath.Clean(TempCertPath), "/"))-1,
				ClusterStateFile),
		},
		Image: alpineSystemImage,
	}
	hostCfg := &container.HostConfig{

		Binds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(host.PrefixPath, "/etc/kubernetes")),
			fmt.Sprintf("%s:/backup:z", etcdSnapshotPath),
		},
		Privileged: true,
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, BundleCertContainer, host.Address, "certificates", prsMap); err != nil {
		return err
	}
	status, err := docker.WaitForContainer(ctx, host.DClient, host.Address, BundleCertContainer)
	if err != nil {
		return err
	}
	if status != 0 {
		containerErrLog, _, err := docker.GetContainerLogsStdoutStderr(ctx, host.DClient, BundleCertContainer, "5", false)
		if err != nil {
			return err
		}
		// removing the container in case of an error too
		if err := docker.RemoveContainer(ctx, host.DClient, host.Address, BundleCertContainer); err != nil {
			return err
		}
		return fmt.Errorf("Failed to run certificate bundle extract, exit status is: %d, container logs: %s", status, containerErrLog)
	}
	log.Infof(ctx, "[certificates] successfully extracted certificate bundle on host [%s] to backup path [%s]", host.Address, TempCertPath)
	return docker.RemoveContainer(ctx, host.DClient, host.Address, BundleCertContainer)
}
