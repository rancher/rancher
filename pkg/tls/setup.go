package tls

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math"
	"math/big"
	"time"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
)

const (
	RancherCommonName = "cattle-ca"
	RancherOrg        = "the-ranch"
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

		caCert, err := newRancherCA(caKey)
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

func newRancherCA(key *rsa.PrivateKey) (*x509.Certificate, error) {
	now := time.Now()
	sn, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	caCert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   RancherCommonName,
			Organization: []string{RancherOrg},
		},
		SerialNumber:          sn,
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(time.Hour * 24 * 365 * 10).UTC(), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certBytes)
}
