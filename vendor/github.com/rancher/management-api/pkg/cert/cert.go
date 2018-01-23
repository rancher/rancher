package cert

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"
)

type CertificateInfo struct {
	Algorithm               string    `json:"algorithm"`
	CN                      string    `json:"cn"`
	Fingerprint             string    `json:"certFingerprint"`
	ExpiresAt               time.Time `json:"expiresAt"`
	IssuedAt                time.Time `json:"issuedAt"`
	Issuer                  string    `json:"issuer"`
	KeySize                 int       `json:"keySize"`
	SerialNumber            string    `json:"serialNumber"`
	SubjectAlternativeNames []string  `json:"subjectAlternativeNames"`
	Version                 int       `json:"version"`
}

func Info(pemCerts, pemKey string) (*CertificateInfo, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, errors.New("failed to parse key, not valid pem format")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read private key")
	}

	rest := []byte(pemCerts)
	for {
		block, rest = pem.Decode(rest)
		var certInfo CertificateInfo

		if block == nil {
			break
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse certificate")
		}

		pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			continue
		}

		if pubKey.N.Cmp(key.N) != 0 {
			continue
		}

		certInfo.Algorithm = "RSA"
		certInfo.Fingerprint = fingerprint(block.Bytes)
		certInfo.CN = cert.Subject.CommonName
		certInfo.ExpiresAt = cert.NotAfter
		certInfo.IssuedAt = cert.NotBefore
		certInfo.Issuer = cert.Issuer.CommonName
		certInfo.KeySize = len(key.N.Bytes())
		certInfo.SerialNumber = cert.SerialNumber.String()
		certInfo.Version = cert.Version

		for _, name := range cert.DNSNames {
			certInfo.SubjectAlternativeNames = append(certInfo.SubjectAlternativeNames, name)
		}

		for _, ip := range cert.IPAddresses {
			certInfo.SubjectAlternativeNames = append(certInfo.SubjectAlternativeNames, ip.String())
		}

		return &certInfo, nil
	}

	return nil, fmt.Errorf("failed to find cert that matched private key")
}

func fingerprint(data []byte) string {
	digest := sha1.Sum(data)
	buf := &bytes.Buffer{}
	for i := 0; i < len(digest); i++ {
		if buf.Len() > 0 {
			buf.WriteString(":")
		}
		buf.WriteString(strings.ToUpper(hex.EncodeToString(digest[i : i+1])))
	}
	return buf.String()
}
