package cluster

import (
	"context"
	"crypto/rsa"
	"fmt"
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
			log.Infof(ctx, "[certificates] Attempting to recover certificates from backup on host [%s]", kubeCluster.EtcdHosts[0].Address)
			kubeCluster.Certificates, err = pki.FetchCertificatesFromHost(ctx, kubeCluster.EtcdHosts, kubeCluster.EtcdHosts[0], kubeCluster.SystemImages.Alpine, kubeCluster.LocalKubeConfigPath, kubeCluster.PrivateRegistriesMap)
			if err != nil {
				return err
			}
			if kubeCluster.Certificates != nil {
				log.Infof(ctx, "[certificates] Certificate backup found on host [%s]", kubeCluster.EtcdHosts[0].Address)
				return nil
			}
			log.Infof(ctx, "[certificates] No Certificate backup found on host [%s]", kubeCluster.EtcdHosts[0].Address)

			kubeCluster.Certificates, err = pki.GenerateRKECerts(ctx, kubeCluster.RancherKubernetesEngineConfig, kubeCluster.LocalKubeConfigPath, "")
			if err != nil {
				return fmt.Errorf("Failed to generate Kubernetes certificates: %v", err)
			}
			log.Infof(ctx, "[certificates] Temporarily saving certs to etcd host [%s]", kubeCluster.EtcdHosts[0].Address)
			if err := pki.DeployCertificatesOnHost(ctx, kubeCluster.EtcdHosts[0], kubeCluster.Certificates, kubeCluster.SystemImages.CertDownloader, pki.TempCertPath, kubeCluster.PrivateRegistriesMap); err != nil {
				return err
			}
			log.Infof(ctx, "[certificates] Saved certs to etcd host [%s]", kubeCluster.EtcdHosts[0].Address)
		}
	}
	return nil
}

func regenerateAPICertificate(c *Cluster, certificates map[string]pki.CertificatePKI) (map[string]pki.CertificatePKI, error) {
	logrus.Debugf("[certificates] Regenerating kubeAPI certificate")
	kubeAPIAltNames := pki.GetAltNames(c.ControlPlaneHosts, c.ClusterDomain, c.KubernetesServiceIP)
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
		if err != nil {
			return nil, err
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
	secretData := map[string][]byte{
		"Certificate": cert.EncodeCertPEM(crt.Certificate),
		"Key":         cert.EncodePrivateKeyPEM(crt.Key),
		"EnvName":     []byte(crt.EnvName),
		"KeyEnvName":  []byte(crt.KeyEnvName),
	}
	if len(crt.Config) > 0 {
		secretData["ConfigEnvName"] = []byte(crt.ConfigEnvName)
		secretData["Config"] = []byte(crt.Config)
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
