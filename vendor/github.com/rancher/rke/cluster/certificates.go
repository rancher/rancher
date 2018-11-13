package cluster

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
)

type RotateCertificatesFlags struct {
	RotateCACerts    bool
	RotateComponents []string
}

func SetUpAuthentication(ctx context.Context, kubeCluster, currentCluster *Cluster, fullState *FullState) error {
	if kubeCluster.Authentication.Strategy == X509AuthenticationProvider {
		kubeCluster.Certificates = fullState.DesiredState.CertificatesBundle
		return nil
	}
	return nil
}

func regenerateAPICertificate(c *Cluster, certificates map[string]pki.CertificatePKI) (map[string]pki.CertificatePKI, error) {
	logrus.Debugf("[certificates] Regenerating kubeAPI certificate")
	kubeAPIAltNames := pki.GetAltNames(c.ControlPlaneHosts, c.ClusterDomain, c.KubernetesServiceIP, c.Authentication.SANs)
	caCrt := certificates[pki.CACertName].Certificate
	caKey := certificates[pki.CACertName].Key
	kubeAPIKey := certificates[pki.KubeAPICertName].Key
	kubeAPICert, _, err := pki.GenerateSignedCertAndKey(caCrt, caKey, true, pki.KubeAPICertName, kubeAPIAltNames, kubeAPIKey, nil)
	if err != nil {
		return nil, err
	}
	certificates[pki.KubeAPICertName] = pki.ToCertObject(pki.KubeAPICertName, "", "", kubeAPICert, kubeAPIKey)
	return certificates, nil
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
		etcdName := pki.GetEtcdCrtName(etcdHost.InternalAddress)
		certificatesNames = append(certificatesNames, etcdName)
	}

	certMap := make(map[string]pki.CertificatePKI)
	for _, certName := range certificatesNames {
		secret, err := k8s.GetSecret(k8sClient, certName)
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
		certMap[pki.ServiceAccountTokenKeyName] = pki.ToCertObject(pki.ServiceAccountTokenKeyName, pki.ServiceAccountTokenKeyName, "", kubeAPICert.Certificate, kubeAPICert.Key)
	}
	log.Infof(ctx, "[certificates] Successfully fetched Cluster certificates from Kubernetes")
	return certMap, nil
}

func saveClusterCerts(ctx context.Context, kubeClient *kubernetes.Clientset, crts map[string]pki.CertificatePKI) error {
	log.Infof(ctx, "[certificates] Save kubernetes certificates as secrets")
	var errgrp errgroup.Group
	for crtName, crt := range crts {
		name := crtName
		certificate := crt
		errgrp.Go(func() error {
			return saveCertToKubernetes(kubeClient, name, certificate)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err

	}
	log.Infof(ctx, "[certificates] Successfully saved certificates as kubernetes secret [%s]", pki.CertificatesSecretName)
	return nil
}

func saveCertToKubernetes(kubeClient *kubernetes.Clientset, crtName string, crt pki.CertificatePKI) error {
	logrus.Debugf("[certificates] Saving certificate [%s] to kubernetes", crtName)
	timeout := make(chan bool, 1)

	// build secret Data
	secretData := make(map[string][]byte)
	if crt.Certificate != nil {
		secretData["Certificate"] = cert.EncodeCertPEM(crt.Certificate)
		secretData["EnvName"] = []byte(crt.EnvName)
		secretData["Path"] = []byte(crt.Path)
	}
	if crt.Key != nil {
		secretData["Key"] = cert.EncodePrivateKeyPEM(crt.Key)
		secretData["KeyEnvName"] = []byte(crt.KeyEnvName)
		secretData["KeyPath"] = []byte(crt.KeyPath)
	}
	if len(crt.Config) > 0 {
		secretData["ConfigEnvName"] = []byte(crt.ConfigEnvName)
		secretData["Config"] = []byte(crt.Config)
		secretData["ConfigPath"] = []byte(crt.ConfigPath)
	}
	go func() {
		for {
			err := k8s.UpdateSecret(kubeClient, secretData, crtName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			timeout <- true
			break
		}
	}()
	select {
	case <-timeout:
		return nil
	case <-time.After(time.Second * KubernetesClientTimeOut):
		return fmt.Errorf("[certificates] Timeout waiting for kubernetes to be ready")
	}
}

func deployBackupCertificates(ctx context.Context, backupHosts []*hosts.Host, kubeCluster *Cluster) error {
	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(backupHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := pki.DeployCertificatesOnHost(ctx, host.(*hosts.Host), kubeCluster.Certificates, kubeCluster.SystemImages.CertDownloader, pki.TempCertPath, kubeCluster.PrivateRegistriesMap)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	return errgrp.Wait()
}

func (c *Cluster) SaveBackupCertificateBundle(ctx context.Context) error {
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(c.getBackupHosts())
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := pki.SaveBackupBundleOnHost(ctx, host.(*hosts.Host), c.SystemImages.Alpine, services.EtcdSnapshotPath, c.PrivateRegistriesMap)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}

	return errgrp.Wait()
}

func (c *Cluster) ExtractBackupCertificateBundle(ctx context.Context) error {
	backupHosts := c.getBackupHosts()
	var errgrp errgroup.Group
	errList := []string{}

	hostsQueue := util.GetObjectQueue(backupHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			for host := range hostsQueue {
				if err := pki.ExtractBackupBundleOnHost(ctx, host.(*hosts.Host), c.SystemImages.Alpine, services.EtcdSnapshotPath, c.PrivateRegistriesMap); err != nil {
					errList = append(errList, fmt.Errorf(
						"Failed to extract certificate bundle on host [%s], please make sure etcd bundle exist in /opt/rke/etcd-snapshots/pki.bundle.tar.gz: %v", host.(*hosts.Host).Address, err).Error())
				}
			}
			return nil
		})
	}

	errgrp.Wait()
	if len(errList) == len(backupHosts) {
		return fmt.Errorf(strings.Join(errList, ","))
	}
	return nil
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

func regenerateAPIAggregationCerts(c *Cluster, certificates map[string]pki.CertificatePKI) (map[string]pki.CertificatePKI, error) {
	logrus.Debugf("[certificates] Regenerating Kubernetes API server aggregation layer requestheader client CA certificates")
	requestHeaderCACrt, requestHeaderCAKey, err := pki.GenerateCACertAndKey(pki.RequestHeaderCACertName, nil)
	if err != nil {
		return nil, err
	}
	certificates[pki.RequestHeaderCACertName] = pki.ToCertObject(pki.RequestHeaderCACertName, "", "", requestHeaderCACrt, requestHeaderCAKey)

	//generate API server proxy client key and certs
	logrus.Debugf("[certificates] Regenerating Kubernetes API server proxy client certificates")
	apiserverProxyClientCrt, apiserverProxyClientKey, err := pki.GenerateSignedCertAndKey(requestHeaderCACrt, requestHeaderCAKey, true, pki.APIProxyClientCertName, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	certificates[pki.APIProxyClientCertName] = pki.ToCertObject(pki.APIProxyClientCertName, "", "", apiserverProxyClientCrt, apiserverProxyClientKey)
	return certificates, nil
}

func RotateRKECertificates(ctx context.Context, c *Cluster, flags ExternalFlags, rotateflags RotateCertificatesFlags, clusterState *FullState) error {
	var (
		serviceAccountTokenKey string
	)
	componentsCertsFuncMap := map[string]pki.GenFunc{
		services.KubeAPIContainerName:        pki.GenerateKubeAPICertificate,
		services.KubeControllerContainerName: pki.GenerateKubeControllerCertificate,
		services.SchedulerContainerName:      pki.GenerateKubeSchedulerCertificate,
		services.KubeproxyContainerName:      pki.GenerateKubeProxyCertificate,
		services.KubeletContainerName:        pki.GenerateKubeNodeCertificate,
		services.EtcdContainerName:           pki.GenerateEtcdCertificates,
	}
	if rotateflags.RotateCACerts {
		// rotate CA cert and RequestHeader CA cert
		if err := pki.GenerateRKECACerts(ctx, c.Certificates, flags.ClusterFilePath, flags.ConfigDir); err != nil {
			return err
		}
		rotateflags.RotateComponents = nil
	}
	for _, k8sComponent := range rotateflags.RotateComponents {
		genFunc := componentsCertsFuncMap[k8sComponent]
		if genFunc != nil {
			if err := genFunc(ctx, c.Certificates, c.RancherKubernetesEngineConfig, flags.ClusterFilePath, flags.ConfigDir, true); err != nil {
				return err
			}
		}
	}
	if len(rotateflags.RotateComponents) == 0 {
		// do not rotate service account token
		if c.Certificates[pki.ServiceAccountTokenKeyName].Key != nil {
			serviceAccountTokenKey = string(cert.EncodePrivateKeyPEM(c.Certificates[pki.ServiceAccountTokenKeyName].Key))
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
				privateKey.(*rsa.PrivateKey))
		}
	}
	clusterState.DesiredState.CertificatesBundle = c.Certificates
	clusterState.DesiredState.RancherKubernetesEngineConfig = &c.RancherKubernetesEngineConfig
	return nil
}

func GetRotateCertsFlags(rotateCACerts bool, components []string) RotateCertificatesFlags {
	return RotateCertificatesFlags{
		RotateCACerts:    rotateCACerts,
		RotateComponents: components,
	}
}
