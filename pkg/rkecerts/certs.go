package rkecerts

import (
	"bytes"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path/filepath"

	rketypes "github.com/rancher/rke/types"

	"context"

	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/rke/rkecerts"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/sirupsen/logrus"
	k8sclientv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	bundleFile = "./management-state/certs/bundle.json"
)

type Bundle struct {
	certs map[string]pki.CertificatePKI
}

func NewBundle(certs map[string]pki.CertificatePKI) *Bundle {
	return &Bundle{
		certs: certs,
	}
}

func Unmarshal(input string) (*Bundle, error) {
	certs, err := rkecerts.LoadString(input)
	return NewBundle(certs), err
}

func (b *Bundle) Certs() map[string]pki.CertificatePKI {
	return b.certs
}

func LoadLocal() (*Bundle, error) {
	f, err := os.Open(bundleFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	certMap, err := rkecerts.Load(f)
	if err != nil {
		return nil, err
	}
	return NewBundle(certMap), nil
}

func Generate(config *rketypes.RancherKubernetesEngineConfig) (*Bundle, error) {
	certs, err := librke.New().GenerateCerts(config)
	if err != nil {
		return nil, err
	}

	return &Bundle{
		certs: certs,
	}, nil
}

// SafeMarshal removes the kube-ca cert key and keyPEM from the cert bundle before marshalling
func (b *Bundle) SafeMarshal() (string, error) {
	output := &bytes.Buffer{}
	certs := b.certs
	if caCert, ok := certs[pki.CACertName]; ok {
		caCert.Key = nil
		caCert.KeyPEM = ""
		certs[pki.CACertName] = caCert
	}
	err := rkecerts.Save(certs, output)
	return output.String(), err

}

func (b *Bundle) ForNode(config *rketypes.RancherKubernetesEngineConfig, nodeAddress string) *Bundle {
	certs := librke.New().GenerateRKENodeCerts(context.Background(), *config, nodeAddress, b.certs)
	return &Bundle{
		certs: certs,
	}
}

func (b *Bundle) ForWindowsNode(rkeconfig *rketypes.RancherKubernetesEngineConfig, nodeAddress string) *Bundle {
	nb := b.ForNode(rkeconfig, nodeAddress)

	certs := make(map[string]pki.CertificatePKI, len(nb.certs))
	for key, cert := range nb.certs {
		if len(cert.Config) != 0 {
			config := &k8sclientv1.Config{}

			if err := yaml.Unmarshal([]byte(cert.Config), config); err == nil {
				clusterAmount := len(config.Clusters)
				for i := 0; i < clusterAmount; i++ {
					cluster := &config.Clusters[i].Cluster

					if len(cluster.CertificateAuthority) != 0 {
						if rkeconfig.WindowsPrefixPath != "" {
							cluster.CertificateAuthority = rkeconfig.WindowsPrefixPath + cluster.CertificateAuthority
						} else {
							cluster.CertificateAuthority = "c:" + cluster.CertificateAuthority
						}
					}
				}

				authInfoAmount := len(config.AuthInfos)
				for i := 0; i < authInfoAmount; i++ {
					authInfo := &config.AuthInfos[i].AuthInfo

					if len(authInfo.ClientCertificate) != 0 {
						if rkeconfig.WindowsPrefixPath != "" {
							authInfo.ClientCertificate = rkeconfig.WindowsPrefixPath + authInfo.ClientCertificate
						} else {
							authInfo.ClientCertificate = "c:" + authInfo.ClientCertificate
						}
					}

					if len(authInfo.ClientKey) != 0 {
						if rkeconfig.WindowsPrefixPath != "" {
							authInfo.ClientKey = rkeconfig.WindowsPrefixPath + authInfo.ClientKey

						} else {
							authInfo.ClientKey = "c:" + authInfo.ClientKey
						}
					}
				}

				if configYamlBytes, err := yaml.Marshal(config); err == nil {
					cert.Config = string(configYamlBytes)
				}
			}
		}

		certs[key] = cert
	}

	return &Bundle{
		certs: certs,
	}
}

func (b *Bundle) SaveLocal() error {
	bundlePath := filepath.Dir(bundleFile)
	if err := os.MkdirAll(bundlePath, 0700); err != nil {
		return err
	}

	f, err := ioutil.TempFile(bundlePath, "bundle-")
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	if err := rkecerts.Save(b.certs, f); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), bundleFile)
}

func (b *Bundle) KubeConfig() string {
	return b.certs["kube-admin"].ConfigPath
}

func (b *Bundle) Explode() error {
	f := &fileWriter{}

	for _, item := range b.certs {
		f.write(item.Path, nil, item.Certificate, nil)
		f.write(item.ConfigPath, []byte(item.Config), nil, nil)
		f.write(item.KeyPath, nil, nil, item.Key)
	}

	return f.err()
}

func (b *Bundle) Changed() bool {
	var newCertPEM string
	for _, item := range b.certs {
		// Skip empty kube-kubelet certificates that are created for other workers
		if item.Name == "" {
			continue
		}
		oldCertPEM, err := ioutil.ReadFile(item.Path)
		if err != nil {
			logrus.Warnf("Unable to read certificate %s: %v", item.Name, err)
			return false
		}
		if item.Certificate != nil {
			newCertPEM = string(cert.EncodeCertPEM(item.Certificate))
		}

		// kube-kubelet certificates will always be different as they are created on-demand, we need to limit replacing them only if its absolutely necessary
		if strings.HasPrefix(item.Name, "kube-kubelet") {
			// Check if expired
			oldCertX509, err := cert.ParseCertsPEM(oldCertPEM)
			if err != nil {
				logrus.Errorf("Error parsing old certificate PEM for [%s]: %v", item.Name, err)
			}
			now := time.Now()
			if len(oldCertX509) > 0 {
				if now.After(oldCertX509[0].NotAfter) {
					logrus.Infof("Bundle changed: now [%v] is after certificate NotAfter [%v] for certificate [%s]", now, oldCertX509[0].NotAfter, item.Name)
					return true
				}
			}
			// Check if AltNames changed
			if newCertPEM != "" {
				newCertX509, err := cert.ParseCertsPEM([]byte(newCertPEM))
				if err != nil {
					logrus.Errorf("Error parsing new certificate PEM for [%s]: %v", item.Name, err)
				}
				if len(newCertX509) > 0 {
					sort.Strings(oldCertX509[0].DNSNames)
					sort.Strings(newCertX509[0].DNSNames)
					if !reflect.DeepEqual(oldCertX509[0].DNSNames, newCertX509[0].DNSNames) || !pki.DeepEqualIPsAltNames(oldCertX509[0].IPAddresses, newCertX509[0].IPAddresses) {
						logrus.Infof("Bundle changed: DNSNames and/or IPAddresses changed for certificate [%s]: oldCert.DNSNames %v, newCert.DNSNames %v, oldCert.IPAddresses %v, newCert.IPAddresses %v", item.Name, oldCertX509[0].DNSNames, newCertX509[0].DNSNames, oldCertX509[0].IPAddresses, newCertX509[0].IPAddresses)
						return true
					}
				}
			}
			continue
		}
		oldCertChecksum := fmt.Sprintf("%x", md5.Sum([]byte(oldCertPEM)))
		newCertChecksum := fmt.Sprintf("%x", md5.Sum([]byte(newCertPEM)))

		if oldCertChecksum != newCertChecksum {
			logrus.Infof("Certificate checksum changed (old: [%s], new [%s]) for [%s]", oldCertChecksum, newCertChecksum, item.Name)
			return true
		}
	}
	return false
}

type fileWriter struct {
	errs []error
}

func (f *fileWriter) write(path string, content []byte, x509cert *x509.Certificate, key *rsa.PrivateKey) {
	if x509cert != nil {
		content = cert.EncodeCertPEM(x509cert)
	}

	if key != nil {
		content = cert.EncodePrivateKeyPEM(key)
	}

	if path == "" || len(content) == 0 {
		return
	}

	existing, err := ioutil.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		f.errs = append(f.errs, err)
	}
	if err := ioutil.WriteFile(path, content, 0600); err != nil {
		f.errs = append(f.errs, err)
	}
}

func (f *fileWriter) err() error {
	return types.NewErrors(f.errs...)
}
