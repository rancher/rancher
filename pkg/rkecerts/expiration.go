package rkecerts

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// CertificateBlockType is a possible value for pem.Block.Type.
const CertificateBlockType = "CERTIFICATE"

type CertificatePKI struct {
	Certificate    *x509.Certificate        `json:"-"`
	Key            *rsa.PrivateKey          `json:"-"`
	CSR            *x509.CertificateRequest `json:"-"`
	CertificatePEM string                   `json:"certificatePEM"`
	KeyPEM         string                   `json:"keyPEM"`
	CSRPEM         string                   `json:"-"`
	Config         string                   `json:"config"`
	Name           string                   `json:"name"`
	CommonName     string                   `json:"commonName"`
	OUName         string                   `json:"ouName"`
	EnvName        string                   `json:"envName"`
	Path           string                   `json:"path"`
	KeyEnvName     string                   `json:"keyEnvName"`
	KeyPath        string                   `json:"keyPath"`
	ConfigEnvName  string                   `json:"configEnvName"`
	ConfigPath     string                   `json:"configPath"`
}

func CleanCertificateBundle(certs map[string]CertificatePKI) {
	for name := range certs {
		if strings.Contains(name, "token") || strings.Contains(name, "header") || strings.Contains(name, "admin") {
			delete(certs, name)
		}
	}
}

func GetCertExpiration(c string) (v32.CertExpiration, error) {
	date, err := GetCertExpirationDate(c)
	if err != nil {
		return v32.CertExpiration{}, err
	}
	return v32.CertExpiration{
		ExpirationDate: date.Format(time.RFC3339),
	}, nil
}

func GetCertExpirationDate(c string) (*time.Time, error) {
	certs, err := ParseCertsPEM([]byte(c))
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, errors.New("no valid certs found")
	}
	return &certs[0].NotAfter, nil
}

// ParseCertsPEM returns the x509.Certificates contained in the given PEM-encoded byte array
// Returns an error if a certificate could not be parsed, or if the data does not contain any certificates
func ParseCertsPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	ok := false
	certs := []*x509.Certificate{}
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		// Only use PEM "CERTIFICATE" blocks without extra headers
		if block.Type != CertificateBlockType || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
		ok = true
	}

	if !ok {
		return certs, errors.New("data does not contain any valid RSA or ECDSA certificates")
	}
	return certs, nil
}
