package rkecerts

import (
	"bytes"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path/filepath"

	"context"

	"fmt"

	"github.com/ghodss/yaml"
	"github.com/rancher/kontainer-engine/drivers/rke/rkecerts"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	k8sclientv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/client-go/util/cert"
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

func Generate(config *v3.RancherKubernetesEngineConfig) (*Bundle, error) {
	certs, err := librke.New().GenerateCerts(config)
	if err != nil {
		return nil, err
	}

	return &Bundle{
		certs: certs,
	}, nil
}

func (b *Bundle) Marshal() (string, error) {
	output := &bytes.Buffer{}
	err := rkecerts.Save(b.certs, output)
	return output.String(), err
}

func (b *Bundle) ForNode(config *v3.RancherKubernetesEngineConfig, nodeAddress string) *Bundle {
	certs := librke.New().GenerateRKENodeCerts(context.Background(), *config, nodeAddress, b.certs)
	return &Bundle{
		certs: certs,
	}
}

func (b *Bundle) ForWindowsNode(config *v3.RancherKubernetesEngineConfig, nodeAddress string) *Bundle {
	nb := b.ForNode(config, nodeAddress)

	certs := make(map[string]pki.CertificatePKI, len(nb.certs))
	for key, cert := range nb.certs {
		if len(cert.Config) != 0 {
			config := &k8sclientv1.Config{}

			if err := yaml.Unmarshal([]byte(cert.Config), config); err == nil {
				clusterAmount := len(config.Clusters)
				for i := 0; i < clusterAmount; i++ {
					cluster := &config.Clusters[i].Cluster

					if len(cluster.CertificateAuthority) != 0 {
						cluster.CertificateAuthority = "c:" + cluster.CertificateAuthority
					}
				}

				authInfoAmount := len(config.AuthInfos)
				for i := 0; i < authInfoAmount; i++ {
					authInfo := &config.AuthInfos[i].AuthInfo

					if len(authInfo.ClientCertificate) != 0 {
						authInfo.ClientCertificate = "c:" + authInfo.ClientCertificate
					}

					if len(authInfo.ClientKey) != 0 {
						authInfo.ClientKey = "c:" + authInfo.ClientKey
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
		oldCertPEM, err := ioutil.ReadFile(item.Path)
		if err != nil {
			logrus.Warnf("Unable to read certificate %s: %v", item.Name, err)
			return false
		}
		if item.Certificate != nil {
			newCertPEM = string(cert.EncodeCertPEM(item.Certificate))
		}
		oldCertChecksum := fmt.Sprintf("%x", md5.Sum([]byte(oldCertPEM)))
		newCertChecksum := fmt.Sprintf("%x", md5.Sum([]byte(newCertPEM)))

		if oldCertChecksum != newCertChecksum {
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
