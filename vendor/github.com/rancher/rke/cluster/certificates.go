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
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
)

func SetUpAuthentication(ctx context.Context, kubeCluster, currentCluster *Cluster) error {
	if kubeCluster.Authentication.Strategy == X509AuthenticationProvider {
		var err error
		if currentCluster != nil {
			kubeCluster.Certificates = currentCluster.Certificates
		} else {
			var backupPlane string
			var backupHosts []*hosts.Host
			if len(kubeCluster.Services.Etcd.ExternalURLs) > 0 {
				backupPlane = ControlPlane
				backupHosts = kubeCluster.ControlPlaneHosts
			} else {
				backupPlane = EtcdPlane
				backupHosts = kubeCluster.EtcdHosts
			}
			log.Infof(ctx, "[certificates] Attempting to recover certificates from backup on [%s] hosts", backupPlane)

			kubeCluster.Certificates, err = fetchBackupCertificates(ctx, backupHosts, kubeCluster)
			if err != nil {
				return err
			}
			if kubeCluster.Certificates != nil {
				log.Infof(ctx, "[certificates] Certificate backup found on [%s] hosts", backupPlane)

				// make sure I have all the etcd certs, We need handle dialer failure for etcd nodes https://github.com/rancher/rancher/issues/12898
				for _, host := range kubeCluster.EtcdHosts {
					certName := pki.GetEtcdCrtName(host.InternalAddress)
					if kubeCluster.Certificates[certName].Certificate == nil {
						if kubeCluster.Certificates, err = pki.RegenerateEtcdCertificate(ctx,
							kubeCluster.Certificates,
							host,
							kubeCluster.EtcdHosts,
							kubeCluster.ClusterDomain,
							kubeCluster.KubernetesServiceIP); err != nil {
							return err
						}
					}
				}
				// this is the case of adding controlplane node on empty cluster with only etcd nodes
				if kubeCluster.Certificates[pki.KubeAdminCertName].Config == "" && len(kubeCluster.ControlPlaneHosts) > 0 {
					if err := rebuildLocalAdminConfig(ctx, kubeCluster); err != nil {
						return err
					}
					kubeCluster.Certificates, err = regenerateAPICertificate(kubeCluster, kubeCluster.Certificates)
					if err != nil {
						return fmt.Errorf("Failed to regenerate KubeAPI certificate %v", err)
					}
				}
				return nil
			}
			log.Infof(ctx, "[certificates] No Certificate backup found on [%s] hosts", backupPlane)

			kubeCluster.Certificates, err = pki.GenerateRKECerts(ctx, kubeCluster.RancherKubernetesEngineConfig, kubeCluster.LocalKubeConfigPath, "")
			if err != nil {
				return fmt.Errorf("Failed to generate Kubernetes certificates: %v", err)
			}

			log.Infof(ctx, "[certificates] Temporarily saving certs to [%s] hosts", backupPlane)
			if err := deployBackupCertificates(ctx, backupHosts, kubeCluster); err != nil {
				return err
			}
			log.Infof(ctx, "[certificates] Saved certs to [%s] hosts", backupPlane)
		}
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

func getClusterCerts(ctx context.Context, kubeClient *kubernetes.Clientset, etcdHosts []*hosts.Host) (map[string]pki.CertificatePKI, error) {
	log.Infof(ctx, "[certificates] Getting Cluster certificates from Kubernetes")
	certificatesNames := []string{
		pki.CACertName,
		pki.KubeAPICertName,
		pki.KubeNodeCertName,
		pki.KubeProxyCertName,
		pki.KubeControllerCertName,
		pki.KubeSchedulerCertName,
		pki.KubeAdminCertName,
	}

	for _, etcdHost := range etcdHosts {
		etcdName := pki.GetEtcdCrtName(etcdHost.InternalAddress)
		certificatesNames = append(certificatesNames, etcdName)
	}

	certMap := make(map[string]pki.CertificatePKI)
	for _, certName := range certificatesNames {
		secret, err := k8s.GetSecret(kubeClient, certName)
		if err != nil && !strings.HasPrefix(certName, "kube-etcd") {
			return nil, err
		}
		// If I can't find an etcd cert, I will not fail and will create it later.
		if secret == nil && strings.HasPrefix(certName, "kube-etcd") {
			certMap[certName] = pki.CertificatePKI{}
			continue
		}

		secretCert, _ := cert.ParseCertsPEM(secret.Data["Certificate"])
		secretKey, _ := cert.ParsePrivateKeyPEM(secret.Data["Key"])
		secretConfig := string(secret.Data["Config"])
		certMap[certName] = pki.CertificatePKI{
			Certificate:   secretCert[0],
			Key:           secretKey.(*rsa.PrivateKey),
			Config:        secretConfig,
			EnvName:       string(secret.Data["EnvName"]),
			ConfigEnvName: string(secret.Data["ConfigEnvName"]),
			KeyEnvName:    string(secret.Data["KeyEnvName"]),
			Path:          string(secret.Data["Path"]),
			KeyPath:       string(secret.Data["KeyPath"]),
			ConfigPath:    string(secret.Data["ConfigPath"]),
		}
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

	for _, host := range backupHosts {
		runHost := host
		errgrp.Go(func() error {
			return pki.DeployCertificatesOnHost(ctx, runHost, kubeCluster.Certificates, kubeCluster.SystemImages.CertDownloader, pki.TempCertPath, kubeCluster.PrivateRegistriesMap)
		})
	}
	return errgrp.Wait()
}

func fetchBackupCertificates(ctx context.Context, backupHosts []*hosts.Host, kubeCluster *Cluster) (map[string]pki.CertificatePKI, error) {
	var err error
	certificates := map[string]pki.CertificatePKI{}
	for _, host := range backupHosts {
		certificates, err = pki.FetchCertificatesFromHost(ctx, kubeCluster.EtcdHosts, host, kubeCluster.SystemImages.Alpine, kubeCluster.LocalKubeConfigPath, kubeCluster.PrivateRegistriesMap)
		if certificates != nil {
			return certificates, nil
		}
	}
	// reporting the last error only.
	return nil, err
}

func fetchCertificatesFromEtcd(ctx context.Context, kubeCluster *Cluster) ([]byte, []byte, error) {
	// Get kubernetes certificates from the etcd hosts
	certificates := map[string]pki.CertificatePKI{}
	var err error
	for _, host := range kubeCluster.EtcdHosts {
		certificates, err = pki.FetchCertificatesFromHost(ctx, kubeCluster.EtcdHosts, host, kubeCluster.SystemImages.Alpine, kubeCluster.LocalKubeConfigPath, kubeCluster.PrivateRegistriesMap)
		if certificates != nil {
			break
		}
	}
	if err != nil || certificates == nil {
		return nil, nil, fmt.Errorf("Failed to fetch certificates from etcd hosts: %v", err)
	}
	clientCert := cert.EncodeCertPEM(certificates[pki.KubeNodeCertName].Certificate)
	clientkey := cert.EncodePrivateKeyPEM(certificates[pki.KubeNodeCertName].Key)
	return clientCert, clientkey, nil
}
