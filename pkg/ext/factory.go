package ext

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/keyutil"
	netutils "k8s.io/utils/net"
)

func GenerateSelfSignedCertKeyWithOpts(host string, expireAfter time.Duration) ([]byte, []byte, error) {
	// valid for an extra check interval before current time to ensure total cert coverage and avoid any issues with clock skew
	validFrom := time.Now().Add(-time.Hour)
	maxAge := expireAfter

	caKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, err
	}
	serial = new(big.Int).Add(serial, big.NewInt(1))
	caTemplate := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s-ca@%d", host, time.Now().Unix()),
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	caCertificate, err := x509.ParseCertificate(caDERBytes)
	if err != nil {
		return nil, nil, err
	}

	priv, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err = cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, err
	}
	serial = new(big.Int).Add(serial, big.NewInt(1))
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s@%d", host, time.Now().Unix()),
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := netutils.ParseIPSloppy(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(cryptorand.Reader, &template, caCertificate, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	// Generate cert, followed by ca
	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: factory.CertificateBlockType, Bytes: derBytes}); err != nil {
		return nil, nil, err
	}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: factory.CertificateBlockType, Bytes: caDERBytes}); err != nil {
		return nil, nil, err
	}

	// Generate key
	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(&keyBuffer, &pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBuffer.Bytes(), nil
}

var _ dynamiclistener.TLSFactory = &SandboxFactory{}

type SandboxFactory struct {
	host string
}

func (s *SandboxFactory) AddCN(secret *v1.Secret, cn ...string) (*v1.Secret, bool, error) {
	logrus.Warnf("SandboxFactory.AddCn test implementation, doing nothing: %v", cn)
	return secret, false, nil
}

func (s *SandboxFactory) Filter(cn ...string) []string {
	logrus.Errorf("SandboxFactory.Filter test implementation, doing nothing: %v", cn)
	return cn
}

func (s *SandboxFactory) Merge(target *v1.Secret, additional *v1.Secret) (*v1.Secret, bool, error) {
	logrus.Errorf("SandboxFactory.Merge test implementation, doing nothing")
	return target, false, nil
}

func (s *SandboxFactory) Regenerate(secret *v1.Secret) (*v1.Secret, error) {
	logrus.Errorf("SandboxFactory.Regenerate test implementation, doing nothing")

	certData, keyData, err := GenerateSelfSignedCertKeyWithOpts(s.host, time.Hour*24*90)
	if err != nil {
		return nil, err
	}

	secret.Data[v1.TLSCertKey] = certData
	secret.Data[v1.TLSPrivateKeyKey] = keyData

	return secret, nil
}

func (s *SandboxFactory) Renew(secret *v1.Secret) (*v1.Secret, error) {
	logrus.Errorf("SandboxFactory.Renew test implementation, doing nothing")

	certData, keyData, err := GenerateSelfSignedCertKeyWithOpts(s.host, time.Hour*24*90)
	if err != nil {
		return nil, err
	}

	secret.Data[v1.TLSCertKey] = certData
	secret.Data[v1.TLSPrivateKeyKey] = keyData

	return secret, nil
}
