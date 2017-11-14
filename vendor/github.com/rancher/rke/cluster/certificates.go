package cluster

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/pki"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
)

func SetUpAuthentication(kubeCluster, currentCluster *Cluster) error {
	if kubeCluster.Authentication.Strategy == X509AuthenticationProvider {
		var err error
		if currentCluster != nil {
			kubeCluster.Certificates = currentCluster.Certificates
		} else {
			kubeCluster.Certificates, err = pki.StartCertificatesGeneration(
				kubeCluster.ControlPlaneHosts,
				kubeCluster.WorkerHosts,
				kubeCluster.ClusterDomain,
				kubeCluster.LocalKubeConfigPath,
				kubeCluster.KubernetesServiceIP)
			if err != nil {
				return fmt.Errorf("Failed to generate Kubernetes certificates: %v", err)
			}
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
	kubeAPICert, err := pki.GenerateCertWithKey(pki.KubeAPICertName, kubeAPIKey, caCrt, caKey, kubeAPIAltNames)
	if err != nil {
		return nil, err
	}
	certificates[pki.KubeAPICertName] = pki.CertificatePKI{
		Certificate:   kubeAPICert,
		Key:           kubeAPIKey,
		Config:        certificates[pki.KubeAPICertName].Config,
		EnvName:       certificates[pki.KubeAPICertName].EnvName,
		ConfigEnvName: certificates[pki.KubeAPICertName].ConfigEnvName,
		KeyEnvName:    certificates[pki.KubeAPICertName].KeyEnvName,
	}
	return certificates, nil
}

func getClusterCerts(kubeClient *kubernetes.Clientset) (map[string]pki.CertificatePKI, error) {
	logrus.Infof("[certificates] Getting Cluster certificates from Kubernetes")
	certificatesNames := []string{
		pki.CACertName,
		pki.KubeAPICertName,
		pki.KubeNodeName,
		pki.KubeProxyName,
		pki.KubeControllerName,
		pki.KubeSchedulerName,
		pki.KubeAdminCommonName,
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
	logrus.Infof("[certificates] Successfully fetched Cluster certificates from Kubernetes")
	return certMap, nil
}

func saveClusterCerts(kubeClient *kubernetes.Clientset, crts map[string]pki.CertificatePKI) error {
	logrus.Infof("[certificates] Save kubernetes certificates as secrets")
	for crtName, crt := range crts {
		err := saveCertToKubernetes(kubeClient, crtName, crt)
		if err != nil {
			return fmt.Errorf("Failed to save certificate [%s] to kubernetes: %v", crtName, err)
		}
	}
	logrus.Infof("[certificates] Successfuly saved certificates as kubernetes secret [%s]", pki.CertificatesSecretName)
	return nil
}

func saveCertToKubernetes(kubeClient *kubernetes.Clientset, crtName string, crt pki.CertificatePKI) error {
	logrus.Debugf("[certificates] Saving certificate [%s] to kubernetes", crtName)
	timeout := make(chan bool, 1)
	go func() {
		for {
			err := k8s.UpdateSecret(kubeClient, "Certificate", cert.EncodeCertPEM(crt.Certificate), crtName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			err = k8s.UpdateSecret(kubeClient, "Key", cert.EncodePrivateKeyPEM(crt.Key), crtName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			err = k8s.UpdateSecret(kubeClient, "EnvName", []byte(crt.EnvName), crtName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			err = k8s.UpdateSecret(kubeClient, "KeyEnvName", []byte(crt.KeyEnvName), crtName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			if len(crt.Config) > 0 {
				err = k8s.UpdateSecret(kubeClient, "ConfigEnvName", []byte(crt.ConfigEnvName), crtName)
				if err != nil {
					time.Sleep(time.Second * 5)
					continue
				}
				err = k8s.UpdateSecret(kubeClient, "Config", []byte(crt.Config), crtName)
				if err != nil {
					time.Sleep(time.Second * 5)
					continue
				}
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
