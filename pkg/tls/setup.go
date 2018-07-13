package tls

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
)

type Storage interface {
	Create(*v3.ListenConfig) (*v3.ListenConfig, error)
	Get(name string, opts metav1.GetOptions) (*v3.ListenConfig, error)
	Update(*v3.ListenConfig) (*v3.ListenConfig, error)
}

func SetupListenConfig(storage Storage, noCACerts bool, lc *v3.ListenConfig) error {
	userCACerts := lc.CACerts

	existing, err := storage.Get(lc.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		existing = nil
	}

	if existing != nil {
		if lc.Cert == "" {
			lc.Cert = existing.Cert
			lc.CACerts = existing.CACerts
			lc.Key = existing.Key
			lc.CAKey = existing.CAKey
			lc.CACert = existing.CACert
			lc.KnownIPs = existing.KnownIPs
			lc.GeneratedCerts = existing.GeneratedCerts
		}
	}

	if (lc.Key == "" || lc.Cert == "") && lc.CACert == "" && lc.Mode != "acme" {
		caKey, err := cert.NewPrivateKey()
		if err != nil {
			return err
		}

		caCert, err := cert.NewSelfSignedCACert(cert.Config{
			CommonName:   "cattle-ca",
			Organization: []string{"the-ranch"},
		}, caKey)
		if err != nil {
			return err
		}

		caCertBuffer := bytes.Buffer{}
		if err := pem.Encode(&caCertBuffer, &pem.Block{
			Type:  cert.CertificateBlockType,
			Bytes: caCert.Raw,
		}); err != nil {
			return err
		}

		caKeyBuffer := bytes.Buffer{}
		if err := pem.Encode(&caKeyBuffer, &pem.Block{
			Type:  cert.RSAPrivateKeyBlockType,
			Bytes: x509.MarshalPKCS1PrivateKey(caKey),
		}); err != nil {
			return err
		}

		lc.CACert = string(caCertBuffer.Bytes())
		lc.CACerts = lc.CACert
		lc.CAKey = string(caKeyBuffer.Bytes())
	}

	if noCACerts || lc.Mode == "acme" {
		lc.CACerts = ""
	} else if userCACerts != "" {
		lc.CACerts = userCACerts
	}

	if existing == nil {
		_, err := storage.Create(lc)
		return err
	}

	lc.ResourceVersion = existing.ResourceVersion
	_, err = storage.Update(lc)
	return err
}
