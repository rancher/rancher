package cluster

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/rke/services"
)

func SetUpAuthentication(ctx context.Context, kubeCluster, currentCluster *Cluster, fullState *FullState) error {
	if kubeCluster.AuthnStrategies[AuthnX509Provider] {
		kubeCluster.Certificates = fullState.DesiredState.CertificatesBundle
		compareCerts(ctx, kubeCluster, currentCluster)
		return nil
	}
	return nil
}

func GetClusterCertsFromKubernetes(ctx context.Context, kubeCluster *Cluster) (map[string]pki.CertificatePKI, error) {
	log.Infof(ctx, "[certificates] Getting Cluster certificates from Kubernetes")

	k8sClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kubernetes Client: %v", err)
	}
	certificatesNames := []string{
		pki.CACertName,
		pki.KubeAPICertName,
		pki.KubeNodeCertName,
		pki.KubeProxyCertName,
		pki.KubeControllerCertName,
		pki.KubeSchedulerCertName,
		pki.KubeAdminCertName,
		pki.APIProxyClientCertName,
		pki.RequestHeaderCACertName,
		pki.ServiceAccountTokenKeyName,
	}

	for _, etcdHost := range kubeCluster.EtcdHosts {
		etcdName := pki.GetCrtNameForHost(etcdHost, pki.EtcdCertName)
		certificatesNames = append(certificatesNames, etcdName)
	}

	certMap := make(map[string]pki.CertificatePKI)
	for _, certName := range certificatesNames {
		secret, err := k8s.GetSystemSecret(k8sClient, certName)
		if err != nil && !strings.HasPrefix(certName, "kube-etcd") &&
			!strings.Contains(certName, pki.RequestHeaderCACertName) &&
			!strings.Contains(certName, pki.APIProxyClientCertName) &&
			!strings.Contains(certName, pki.ServiceAccountTokenKeyName) {
			return nil, err
		}
		// If I can't find an etcd, requestheader, or proxy client cert, I will not fail and will create it later.
		if (secret == nil || secret.Data == nil) &&
			(strings.HasPrefix(certName, "kube-etcd") ||
				strings.Contains(certName, pki.RequestHeaderCACertName) ||
				strings.Contains(certName, pki.APIProxyClientCertName) ||
				strings.Contains(certName, pki.ServiceAccountTokenKeyName)) {
			certMap[certName] = pki.CertificatePKI{}
			continue
		}

		secretCert, err := cert.ParseCertsPEM(secret.Data["Certificate"])
		if err != nil {
			return nil, fmt.Errorf("Failed to parse certificate of %s: %v", certName, err)
		}
		secretKey, err := cert.ParsePrivateKeyPEM(secret.Data["Key"])
		if err != nil {
			return nil, fmt.Errorf("Failed to parse private key of %s: %v", certName, err)
		}
		secretConfig := string(secret.Data["Config"])
		if len(secretCert) == 0 || secretKey == nil {
			return nil, fmt.Errorf("certificate or key of %s is not found", certName)
		}
		certificatePEM := string(cert.EncodeCertPEM(secretCert[0]))
		keyPEM := string(cert.EncodePrivateKeyPEM(secretKey.(*rsa.PrivateKey)))

		certMap[certName] = pki.CertificatePKI{
			Certificate:    secretCert[0],
			Key:            secretKey.(*rsa.PrivateKey),
			CertificatePEM: certificatePEM,
			KeyPEM:         keyPEM,
			Config:         secretConfig,
			EnvName:        string(secret.Data["EnvName"]),
			ConfigEnvName:  string(secret.Data["ConfigEnvName"]),
			KeyEnvName:     string(secret.Data["KeyEnvName"]),
			Path:           string(secret.Data["Path"]),
			KeyPath:        string(secret.Data["KeyPath"]),
			ConfigPath:     string(secret.Data["ConfigPath"]),
		}
	}
	// Handle service account token key issue
	kubeAPICert := certMap[pki.KubeAPICertName]
	if certMap[pki.ServiceAccountTokenKeyName].Key == nil {
		log.Infof(ctx, "[certificates] Creating service account token key")
		certMap[pki.ServiceAccountTokenKeyName] = pki.ToCertObject(pki.ServiceAccountTokenKeyName, pki.ServiceAccountTokenKeyName, "", kubeAPICert.Certificate, kubeAPICert.Key, nil)
	}
	log.Infof(ctx, "[certificates] Successfully fetched Cluster certificates from Kubernetes")
	return certMap, nil
}

func (c *Cluster) getBackupHosts() []*hosts.Host {
	var backupHosts []*hosts.Host
	if len(c.Services.Etcd.ExternalURLs) > 0 {
		backupHosts = c.ControlPlaneHosts
	} else {
		// Save certificates on etcd and controlplane hosts
		backupHosts = hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, nil)
	}
	return backupHosts
}

func RotateRKECertificates(ctx context.Context, c *Cluster, flags ExternalFlags, clusterState *FullState) error {
	var (
		serviceAccountTokenKey string
	)
	componentsCertsFuncMap := map[string][]pki.GenFunc{
		services.KubeAPIContainerName:        []pki.GenFunc{pki.GenerateKubeAPICertificate},
		services.KubeControllerContainerName: []pki.GenFunc{pki.GenerateKubeControllerCertificate},
		services.SchedulerContainerName:      []pki.GenFunc{pki.GenerateKubeSchedulerCertificate},
		services.KubeproxyContainerName:      []pki.GenFunc{pki.GenerateKubeProxyCertificate},
		services.KubeletContainerName:        []pki.GenFunc{pki.GenerateKubeNodeCertificate},
		services.EtcdContainerName:           []pki.GenFunc{pki.GenerateEtcdCertificates},
	}
	if c.IsKubeletGenerateServingCertificateEnabled() {
		componentsCertsFuncMap[services.KubeletContainerName] = append(componentsCertsFuncMap[services.KubeletContainerName], pki.GenerateKubeletCertificate)
	}
	rotateFlags := c.RancherKubernetesEngineConfig.RotateCertificates
	if rotateFlags.CACertificates {
		// rotate CA cert and RequestHeader CA cert
		if err := pki.GenerateRKECACerts(ctx, c.Certificates, flags.ClusterFilePath, flags.ConfigDir); err != nil {
			return err
		}
		rotateFlags.Services = nil
	}
	for _, k8sComponent := range rotateFlags.Services {
		genFunctions := componentsCertsFuncMap[k8sComponent]
		if genFunctions != nil {
			for _, genFunc := range genFunctions {
				if err := genFunc(ctx, c.Certificates, c.RancherKubernetesEngineConfig, flags.ClusterFilePath, flags.ConfigDir, true); err != nil {
					return err
				}
			}
		}
	}
	// to handle kontainer engine sending empty string for services
	if len(rotateFlags.Services) == 0 || (len(rotateFlags.Services) == 1 && rotateFlags.Services[0] == "") {
		// do not rotate service account token
		if c.Certificates[pki.ServiceAccountTokenKeyName].Key != nil {
			serviceAccountTokenKey = string(cert.EncodePrivateKeyPEM(c.Certificates[pki.ServiceAccountTokenKeyName].Key))
		}
		// check for legacy clusters prior to requestheaderca
		if c.Certificates[pki.RequestHeaderCACertName].Certificate == nil {
			if err := pki.GenerateRKERequestHeaderCACert(ctx, c.Certificates, flags.ClusterFilePath, flags.ConfigDir); err != nil {
				return err
			}
		}
		if err := pki.GenerateRKEServicesCerts(ctx, c.Certificates, c.RancherKubernetesEngineConfig, flags.ClusterFilePath, flags.ConfigDir, true); err != nil {
			return err
		}
		if serviceAccountTokenKey != "" {
			privateKey, err := cert.ParsePrivateKeyPEM([]byte(serviceAccountTokenKey))
			if err != nil {
				return err
			}
			c.Certificates[pki.ServiceAccountTokenKeyName] = pki.ToCertObject(
				pki.ServiceAccountTokenKeyName,
				pki.ServiceAccountTokenKeyName,
				"",
				c.Certificates[pki.ServiceAccountTokenKeyName].Certificate,
				privateKey.(*rsa.PrivateKey), nil)
		}
	}
	clusterState.DesiredState.CertificatesBundle = c.Certificates
	return nil
}

func GetClusterCertsFromNodes(ctx context.Context, kubeCluster *Cluster) (map[string]pki.CertificatePKI, error) {
	log.Infof(ctx, "[certificates] Fetching kubernetes certificates from nodes")
	var err error
	backupHosts := hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, nil)
	certificates := map[string]pki.CertificatePKI{}
	for _, host := range backupHosts {
		certificates, err = pki.FetchCertificatesFromHost(ctx, kubeCluster.EtcdHosts, host, kubeCluster.SystemImages.Alpine, kubeCluster.LocalKubeConfigPath, kubeCluster.PrivateRegistriesMap)
		if certificates != nil {
			// Handle service account token key issue
			kubeAPICert := certificates[pki.KubeAPICertName]
			if certificates[pki.ServiceAccountTokenKeyName].Key == nil {
				log.Infof(ctx, "[certificates] Creating service account token key")
				certificates[pki.ServiceAccountTokenKeyName] = pki.ToCertObject(pki.ServiceAccountTokenKeyName, pki.ServiceAccountTokenKeyName, "", kubeAPICert.Certificate, kubeAPICert.Key, nil)
			}
			return certificates, nil
		}
	}
	// reporting the last error only.
	return nil, err
}

func compareCerts(ctx context.Context, kubeCluster, currentCluster *Cluster) {
	// check if relevant certs were changed and if so set force deploy to true
	// to deploy certs with new SANs
	if currentCluster != nil {
		for _, certName := range []string{
			pki.KubeAPICertName,
			pki.EtcdCertName,
		} {
			currentCert := currentCluster.Certificates[certName]
			desiredCert := kubeCluster.Certificates[certName]
			if desiredCert.CertificatePEM != currentCert.CertificatePEM {
				log.Infof(ctx, "[certificates] %s certificate changed, force deploying certs", certName)
				kubeCluster.ForceDeployCerts = true
				return
			}
		}
	}
}

func (c *Cluster) IsKubeletGenerateServingCertificateEnabled() bool {
	if c == nil {
		return false
	}
	if c.Services.Kubelet.GenerateServingCertificate {
		return true
	}
	return false
}
