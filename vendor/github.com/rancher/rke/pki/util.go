package pki

import (
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/client-go/util/cert"
)

func GenerateSignedCertAndKey(
	caCrt *x509.Certificate,
	caKey *rsa.PrivateKey,
	serverCrt bool,
	commonName string,
	altNames *cert.AltNames,
	reusedKey *rsa.PrivateKey,
	orgs []string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Generate a generic signed certificate
	var rootKey *rsa.PrivateKey
	var err error
	rootKey = reusedKey
	if reusedKey == nil {
		rootKey, err = cert.NewPrivateKey()
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to generate private key for %s certificate: %v", commonName, err)
		}
	}
	usages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if serverCrt {
		usages = append(usages, x509.ExtKeyUsageServerAuth)
	}
	if altNames == nil {
		altNames = &cert.AltNames{}
	}
	caConfig := cert.Config{
		CommonName:   commonName,
		Organization: orgs,
		Usages:       usages,
		AltNames:     *altNames,
	}
	clientCert, err := newSignedCert(caConfig, rootKey, caCrt, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate %s certificate: %v", commonName, err)
	}
	return clientCert, rootKey, nil
}

func GenerateCACertAndKey(commonName string, privateKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
	var err error
	rootKey := privateKey
	if rootKey == nil {
		rootKey, err = cert.NewPrivateKey()
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to generate private key for CA certificate: %v", err)
		}
	}
	caConfig := cert.Config{
		CommonName: commonName,
	}
	kubeCACert, err := cert.NewSelfSignedCACert(caConfig, rootKey)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate CA certificate: %v", err)
	}

	return kubeCACert, rootKey, nil
}

func GetAltNames(cpHosts []*hosts.Host, clusterDomain string, KubernetesServiceIP net.IP, SANs []string) *cert.AltNames {
	ips := []net.IP{}
	dnsNames := []string{}
	for _, host := range cpHosts {
		// Check if node address is a valid IP
		if nodeIP := net.ParseIP(host.Address); nodeIP != nil {
			ips = append(ips, nodeIP)
		} else {
			dnsNames = append(dnsNames, host.Address)
		}

		// Check if node internal address is a valid IP
		if len(host.InternalAddress) != 0 && host.InternalAddress != host.Address {
			if internalIP := net.ParseIP(host.InternalAddress); internalIP != nil {
				ips = append(ips, internalIP)
			} else {
				dnsNames = append(dnsNames, host.InternalAddress)
			}
		}
		// Add hostname to the ALT dns names
		if len(host.HostnameOverride) != 0 && host.HostnameOverride != host.Address {
			dnsNames = append(dnsNames, host.HostnameOverride)
		}
	}

	for _, host := range SANs {
		// Check if node address is a valid IP
		if nodeIP := net.ParseIP(host); nodeIP != nil {
			ips = append(ips, nodeIP)
		} else {
			dnsNames = append(dnsNames, host)
		}
	}

	ips = append(ips, net.ParseIP("127.0.0.1"))
	ips = append(ips, KubernetesServiceIP)
	dnsNames = append(dnsNames, []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc." + clusterDomain,
	}...)
	return &cert.AltNames{
		IPs:      ips,
		DNSNames: dnsNames,
	}
}

func (c *CertificatePKI) ToEnv() []string {
	env := []string{}
	if c.Key != nil {
		env = append(env, c.KeyToEnv())
	}
	if c.Certificate != nil {
		env = append(env, c.CertToEnv())
	}
	if c.Config != "" && c.ConfigEnvName != "" {
		env = append(env, c.ConfigToEnv())
	}
	return env
}

func (c *CertificatePKI) CertToEnv() string {
	encodedCrt := cert.EncodeCertPEM(c.Certificate)
	return fmt.Sprintf("%s=%s", c.EnvName, string(encodedCrt))
}

func (c *CertificatePKI) KeyToEnv() string {
	encodedKey := cert.EncodePrivateKeyPEM(c.Key)
	return fmt.Sprintf("%s=%s", c.KeyEnvName, string(encodedKey))
}

func (c *CertificatePKI) ConfigToEnv() string {
	return fmt.Sprintf("%s=%s", c.ConfigEnvName, c.Config)
}

func getEnvFromName(name string) string {
	return strings.Replace(strings.ToUpper(name), "-", "_", -1)
}

func getKeyEnvFromEnv(env string) string {
	return fmt.Sprintf("%s_KEY", env)
}

func getConfigEnvFromEnv(env string) string {
	return fmt.Sprintf("KUBECFG_%s", env)
}

func GetEtcdCrtName(address string) string {
	newAddress := strings.Replace(address, ".", "-", -1)
	return fmt.Sprintf("%s-%s", EtcdCertName, newAddress)
}

func GetCertPath(name string) string {
	return fmt.Sprintf("%s%s.pem", CertPathPrefix, name)
}

func GetKeyPath(name string) string {
	return fmt.Sprintf("%s%s-key.pem", CertPathPrefix, name)
}

func GetConfigPath(name string) string {
	return fmt.Sprintf("%skubecfg-%s.yaml", CertPathPrefix, name)
}

func GetCertTempPath(name string) string {
	return fmt.Sprintf("%s%s.pem", TempCertPath, name)
}

func GetKeyTempPath(name string) string {
	return fmt.Sprintf("%s%s-key.pem", TempCertPath, name)
}

func GetConfigTempPath(name string) string {
	return fmt.Sprintf("%skubecfg-%s.yaml", TempCertPath, name)
}

func ToCertObject(componentName, commonName, ouName string, certificate *x509.Certificate, key *rsa.PrivateKey) CertificatePKI {
	var config, configPath, configEnvName string
	if len(commonName) == 0 {
		commonName = getDefaultCN(componentName)
	}

	envName := getEnvFromName(componentName)
	keyEnvName := getKeyEnvFromEnv(envName)
	caCertPath := GetCertPath(CACertName)
	path := GetCertPath(componentName)
	keyPath := GetKeyPath(componentName)
	certificatePEM := string(cert.EncodeCertPEM(certificate))
	keyPEM := string(cert.EncodePrivateKeyPEM(key))

	if componentName != CACertName && componentName != KubeAPICertName && !strings.Contains(componentName, EtcdCertName) && componentName != ServiceAccountTokenKeyName {
		config = getKubeConfigX509("https://127.0.0.1:6443", "local", componentName, caCertPath, path, keyPath)
		configPath = GetConfigPath(componentName)
		configEnvName = getConfigEnvFromEnv(envName)
	}

	return CertificatePKI{
		Certificate:    certificate,
		Key:            key,
		CertificatePEM: certificatePEM,
		KeyPEM:         keyPEM,
		Config:         config,
		Name:           componentName,
		CommonName:     commonName,
		OUName:         ouName,
		EnvName:        envName,
		KeyEnvName:     keyEnvName,
		ConfigEnvName:  configEnvName,
		Path:           path,
		KeyPath:        keyPath,
		ConfigPath:     configPath,
	}
}

func getDefaultCN(name string) string {
	return fmt.Sprintf("system:%s", name)
}

func getControlCertKeys() []string {
	return []string{
		CACertName,
		KubeAPICertName,
		ServiceAccountTokenKeyName,
		KubeControllerCertName,
		KubeSchedulerCertName,
		KubeProxyCertName,
		KubeNodeCertName,
		EtcdClientCertName,
		EtcdClientCACertName,
		RequestHeaderCACertName,
		APIProxyClientCertName,
	}
}

func getWorkerCertKeys() []string {
	return []string{
		CACertName,
		KubeProxyCertName,
		KubeNodeCertName,
	}
}

func getEtcdCertKeys(rkeNodes []v3.RKEConfigNode, etcdRole string) []string {
	certList := []string{
		CACertName,
		KubeProxyCertName,
		KubeNodeCertName,
	}
	etcdHosts := hosts.NodesToHosts(rkeNodes, etcdRole)
	for _, host := range etcdHosts {
		certList = append(certList, GetEtcdCrtName(host.InternalAddress))
	}
	return certList

}

func GetKubernetesServiceIP(serviceClusterRange string) (net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(serviceClusterRange)
	if err != nil {
		return nil, fmt.Errorf("Failed to get kubernetes service IP from Kube API option [service_cluster_ip_range]: %v", err)
	}
	ip = ip.Mask(ipnet.Mask)
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
	return ip, nil
}

func GetLocalKubeConfig(configPath, configDir string) string {
	baseDir := filepath.Dir(configPath)
	if len(configDir) > 0 {
		baseDir = filepath.Dir(configDir)
	}
	fileName := filepath.Base(configPath)
	baseDir += "/"
	return fmt.Sprintf("%s%s%s", baseDir, KubeAdminConfigPrefix, fileName)
}

func strCrtToEnv(crtName, crt string) string {
	return fmt.Sprintf("%s=%s", getEnvFromName(crtName), crt)
}

func strKeyToEnv(crtName, key string) string {
	envName := getEnvFromName(crtName)
	return fmt.Sprintf("%s=%s", getKeyEnvFromEnv(envName), key)
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

// Overriding k8s.io/client-go/util/cert.NewSignedCert function to extend the expiration date to 10 years instead of 1 year
func newSignedCert(cfg cert.Config, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}
	if len(cfg.Usages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(duration365d * 10).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usages,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func isFileNotFoundErr(e error) bool {
	if strings.Contains(e.Error(), "no such file or directory") ||
		strings.Contains(e.Error(), "Could not find the file") ||
		strings.Contains(e.Error(), "No such container:path:") {
		return true
	}
	return false
}

func deepEqualIPsAltNames(oldIPs, newIPs []net.IP) bool {
	if len(oldIPs) != len(newIPs) {
		return false
	}
	oldIPsStrings := make([]string, len(oldIPs))
	newIPsStrings := make([]string, len(newIPs))
	for i := range oldIPs {
		oldIPsStrings = append(oldIPsStrings, oldIPs[i].String())
		newIPsStrings = append(newIPsStrings, newIPs[i].String())
	}
	return reflect.DeepEqual(oldIPsStrings, newIPsStrings)
}

func TransformPEMToObject(in map[string]CertificatePKI) map[string]CertificatePKI {
	out := map[string]CertificatePKI{}
	for k, v := range in {
		certs, _ := cert.ParseCertsPEM([]byte(v.CertificatePEM))
		key, _ := cert.ParsePrivateKeyPEM([]byte(v.KeyPEM))
		o := CertificatePKI{
			ConfigEnvName:  v.ConfigEnvName,
			Name:           v.Name,
			Config:         v.Config,
			CommonName:     v.CommonName,
			OUName:         v.OUName,
			EnvName:        v.EnvName,
			Path:           v.Path,
			KeyEnvName:     v.KeyEnvName,
			KeyPath:        v.KeyPath,
			ConfigPath:     v.ConfigPath,
			Certificate:    certs[0],
			Key:            key.(*rsa.PrivateKey),
			CertificatePEM: v.CertificatePEM,
			KeyPEM:         v.KeyPEM,
		}
		out[k] = o
	}
	return out
}
