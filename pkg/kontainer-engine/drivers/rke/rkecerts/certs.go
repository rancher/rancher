package rkecerts

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"

	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
)

type savedCertificatePKI struct {
	pki.CertificatePKI
	CertPEM string
	KeyPEM  string
}

func LoadString(input string) (map[string]pki.CertificatePKI, error) {
	return Load(bytes.NewBufferString(input))
}

func Load(f io.Reader) (map[string]pki.CertificatePKI, error) {
	saved := map[string]savedCertificatePKI{}
	if err := json.NewDecoder(f).Decode(&saved); err != nil {
		return nil, err
	}

	certs := map[string]pki.CertificatePKI{}

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

		certs[name] = savedCert.CertificatePKI
	}

	return certs, nil
}

func ToString(certs map[string]pki.CertificatePKI) (string, error) {
	output := &bytes.Buffer{}
	err := Save(certs, output)
	return output.String(), err
}

func Save(certs map[string]pki.CertificatePKI, w io.Writer) error {
	toSave := map[string]savedCertificatePKI{}

	for name, bundleCert := range certs {
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
