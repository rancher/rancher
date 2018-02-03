package pki

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"net"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"k8s.io/client-go/util/cert"
)

type CertificatePKI struct {
	Certificate   *x509.Certificate
	Key           *rsa.PrivateKey
	Config        string
	Name          string
	CommonName    string
	OUName        string
	EnvName       string
	Path          string
	KeyEnvName    string
	KeyPath       string
	ConfigEnvName string
	ConfigPath    string
}

// StartCertificatesGeneration ...
func StartCertificatesGeneration(ctx context.Context, cpHosts, etcdHosts []*hosts.Host, clusterDomain, localConfigPath string, KubernetesServiceIP net.IP) (map[string]CertificatePKI, error) {
	log.Infof(ctx, "[certificates] Generating kubernetes certificates")
	certs, err := generateCerts(ctx, cpHosts, etcdHosts, clusterDomain, localConfigPath, KubernetesServiceIP)
	if err != nil {
		return nil, err
	}
	return certs, nil
}

func generateCerts(ctx context.Context, cpHosts, etcdHosts []*hosts.Host, clusterDomain, localConfigPath string, KubernetesServiceIP net.IP) (map[string]CertificatePKI, error) {
	certs := make(map[string]CertificatePKI)
	// generate CA certificate and key
	log.Infof(ctx, "[certificates] Generating CA kubernetes certificates")
	caCrt, caKey, err := generateCACertAndKey()
	if err != nil {
		return nil, err
	}
	certs[CACertName] = ToCertObject(CACertName, "", "", caCrt, caKey)

	// generate API certificate and key
	log.Infof(ctx, "[certificates] Generating Kubernetes API server certificates")
	kubeAPIAltNames := GetAltNames(cpHosts, clusterDomain, KubernetesServiceIP)
	kubeAPICrt, kubeAPIKey, err := GenerateSignedCertAndKey(caCrt, caKey, true, KubeAPICertName, kubeAPIAltNames, nil, nil)
	if err != nil {
		return nil, err
	}
	certs[KubeAPICertName] = ToCertObject(KubeAPICertName, "", "", kubeAPICrt, kubeAPIKey)

	// generate Kube controller-manager certificate and key
	log.Infof(ctx, "[certificates] Generating Kube Controller certificates")
	kubeControllerCrt, kubeControllerKey, err := GenerateSignedCertAndKey(caCrt, caKey, false, getDefaultCN(KubeControllerCertName), nil, nil, nil)
	if err != nil {
		return nil, err
	}
	certs[KubeControllerCertName] = ToCertObject(KubeControllerCertName, "", "", kubeControllerCrt, kubeControllerKey)

	// generate Kube scheduler certificate and key
	log.Infof(ctx, "[certificates] Generating Kube Scheduler certificates")
	kubeSchedulerCrt, kubeSchedulerKey, err := GenerateSignedCertAndKey(caCrt, caKey, false, getDefaultCN(KubeSchedulerCertName), nil, nil, nil)
	if err != nil {
		return nil, err
	}
	certs[KubeSchedulerCertName] = ToCertObject(KubeSchedulerCertName, "", "", kubeSchedulerCrt, kubeSchedulerKey)

	// generate Kube Proxy certificate and key
	log.Infof(ctx, "[certificates] Generating Kube Proxy certificates")
	kubeProxyCrt, kubeProxyKey, err := GenerateSignedCertAndKey(caCrt, caKey, false, getDefaultCN(KubeProxyCertName), nil, nil, nil)
	if err != nil {
		return nil, err
	}
	certs[KubeProxyCertName] = ToCertObject(KubeProxyCertName, "", "", kubeProxyCrt, kubeProxyKey)

	// generate Kubelet certificate and key
	log.Infof(ctx, "[certificates] Generating Node certificate")
	nodeCrt, nodeKey, err := GenerateSignedCertAndKey(caCrt, caKey, false, KubeNodeCommonName, nil, nil, []string{KubeNodeOrganizationName})
	if err != nil {
		return nil, err
	}
	certs[KubeNodeCertName] = ToCertObject(KubeNodeCertName, KubeNodeCommonName, KubeNodeOrganizationName, nodeCrt, nodeKey)

	// generate Admin certificate and key
	log.Infof(ctx, "[certificates] Generating admin certificates and kubeconfig")
	kubeAdminCrt, kubeAdminKey, err := GenerateSignedCertAndKey(caCrt, caKey, false, KubeAdminCertName, nil, nil, []string{KubeAdminOrganizationName})
	if err != nil {
		return nil, err
	}
	kubeAdminConfig := GetKubeConfigX509WithData(
		"https://"+cpHosts[0].Address+":6443",
		KubeAdminCertName,
		string(cert.EncodeCertPEM(caCrt)),
		string(cert.EncodeCertPEM(kubeAdminCrt)),
		string(cert.EncodePrivateKeyPEM(kubeAdminKey)))

	kubeAdminCertObj := ToCertObject(KubeAdminCertName, KubeAdminCertName, KubeAdminOrganizationName, kubeAdminCrt, kubeAdminKey)
	kubeAdminCertObj.Config = kubeAdminConfig
	kubeAdminCertObj.ConfigPath = localConfigPath
	certs[KubeAdminCertName] = kubeAdminCertObj

	etcdAltNames := GetAltNames(etcdHosts, clusterDomain, KubernetesServiceIP)
	for _, host := range etcdHosts {
		log.Infof(ctx, "[certificates] Generating etcd-%s certificate and key", host.InternalAddress)
		etcdCrt, etcdKey, err := GenerateSignedCertAndKey(caCrt, caKey, true, EtcdCertName, etcdAltNames, nil, nil)
		if err != nil {
			return nil, err
		}
		etcdName := GetEtcdCrtName(host.InternalAddress)
		certs[etcdName] = ToCertObject(etcdName, "", "", etcdCrt, etcdKey)
	}

	return certs, nil
}

func RegenerateEtcdCertificate(
	ctx context.Context,
	crtMap map[string]CertificatePKI,
	etcdHost *hosts.Host,
	etcdHosts []*hosts.Host,
	clusterDomain string,
	KubernetesServiceIP net.IP) (map[string]CertificatePKI, error) {

	log.Infof(ctx, "[certificates] Regenerating new etcd-%s certificate and key", etcdHost.InternalAddress)
	caCrt := crtMap[CACertName].Certificate
	caKey := crtMap[CACertName].Key
	etcdAltNames := GetAltNames(etcdHosts, clusterDomain, KubernetesServiceIP)

	etcdCrt, etcdKey, err := GenerateSignedCertAndKey(caCrt, caKey, true, EtcdCertName, etcdAltNames, nil, nil)
	if err != nil {
		return nil, err
	}
	etcdName := GetEtcdCrtName(etcdHost.InternalAddress)
	crtMap[etcdName] = ToCertObject(etcdName, "", "", etcdCrt, etcdKey)
	log.Infof(ctx, "[certificates] Successfully generated new etcd-%s certificate and key", etcdHost.InternalAddress)
	return crtMap, nil
}
