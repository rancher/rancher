package rkecerts

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"crypto/x509"

	"bytes"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/client-go/util/cert"
)

const (
	bundleFile = "./management-state/certs/bundle.json"
)

var (
	kubeconfigReplace = map[string]bool{
		"/etc/kubernetes/ssl/kubecfg-kube-node.yaml":  true,
		"/etc/kubernetes/ssl/kubecfg-kube-proxy.yaml": true,
	}
)

type savedCertificatePKI struct {
	pki.CertificatePKI
	CertPEM string
	KeyPEM  string
}

type Bundle struct {
	certs map[string]pki.CertificatePKI
}

func Unmarshal(input string) (*Bundle, error) {
	return load(bytes.NewBufferString(input))
}

func (b *Bundle) Certs() map[string]pki.CertificatePKI {
	return b.certs
}

func Load() (*Bundle, error) {
	f, err := os.Open(bundleFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return load(f)
}

func load(f io.Reader) (*Bundle, error) {
	saved := map[string]savedCertificatePKI{}
	if err := json.NewDecoder(f).Decode(&saved); err != nil {
		return nil, err
	}

	bundle := &Bundle{
		certs: map[string]pki.CertificatePKI{},
	}

	for name, savedCert := range saved {
		if savedCert.CertPEM != "" {
			certs, err := cert.ParseCertsPEM([]byte(savedCert.CertPEM))
			if err != nil {
				return nil, err
			}

			if len(certs) == 0 {
				return nil, fmt.Errorf("failed to parse certs, 0 found")
			}

			savedCert.Certificate = certs[0]
		}

		if savedCert.KeyPEM != "" {
			key, err := cert.ParsePrivateKeyPEM([]byte(savedCert.KeyPEM))
			if err != nil {
				return nil, err
			}
			savedCert.Key = key.(*rsa.PrivateKey)
		}

		bundle.certs[name] = savedCert.CertificatePKI
	}

	return bundle, nil
}

func Generate(ctx context.Context, config *v3.RancherKubernetesEngineConfig) (*Bundle, error) {
	certs, err := pki.GenerateRKECerts(ctx, *config, "", "")
	if err != nil {
		return nil, err
	}

	return &Bundle{
		certs: certs,
	}, nil
}

func (b *Bundle) Marshal() (string, error) {
	output := &bytes.Buffer{}
	err := b.save(output)
	return output.String(), err
}

func (b *Bundle) save(w io.Writer) error {
	toSave := map[string]savedCertificatePKI{}

	for name, bundleCert := range b.certs {
		toSaveCert := savedCertificatePKI{
			CertificatePKI: bundleCert,
		}

		if toSaveCert.Certificate != nil {
			toSaveCert.CertPEM = string(cert.EncodeCertPEM(toSaveCert.Certificate))
		}

		if toSaveCert.Key != nil {
			toSaveCert.KeyPEM = string(cert.EncodePrivateKeyPEM(toSaveCert.Key))
		}

		toSaveCert.Certificate = nil
		toSaveCert.Key = nil

		toSave[name] = toSaveCert
	}

	return json.NewEncoder(w).Encode(toSave)
}

func (b *Bundle) ForNode(config *v3.RancherKubernetesEngineConfig, node *v3.RKEConfigNode, server, token string) (*Bundle, error) {
	certs := pki.GenerateRKENodeCerts(context.Background(), *config, node.Address, b.certs)

	for name, cert := range certs {
		if kubeconfigReplace[cert.ConfigPath] {
			newCfg, err := kubeconfig.ForBasic(server, node.HostnameOverride, token)
			if err != nil {
				return nil, err
			}
			cert.Config = newCfg
			certs[name] = cert
		}
	}

	return &Bundle{
		certs: certs,
	}, nil
}

func (b *Bundle) Save() error {
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

	if err := b.save(f); err != nil {
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

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		f.errs = append(f.errs, err)
	}
	if err := ioutil.WriteFile(path, content, 0600); err != nil {
		f.errs = append(f.errs, err)
	}
}

func (f *fileWriter) err() error {
	return types.NewErrors(f.errs)
}
