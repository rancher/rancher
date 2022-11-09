package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/stretchr/testify/assert"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestKubeletCertificateNeedsRegeneration(t *testing.T) {

	correctIPAddress := "167.71.188.113"
	correctHostName := "cert-test1"
	testCertExpiryTime := time.Now().AddDate(10, 0, 3)

	type testcase struct {
		testName      string
		testIPAddress string
		testHostName  string
		testTime      time.Time
		want          bool
	}

	testCases := []testcase{
		{
			testName:      "All Cert Properties Are Valid",
			testIPAddress: correctIPAddress,
			testHostName:  correctHostName,
			testTime:      time.Now(),
			want:          false,
		},
		{
			testName:      "Cert Contains Invalid IP Address",
			testIPAddress: "192.168.1.1",
			testHostName:  correctHostName,
			testTime:      time.Now(),
			want:          true,
		},
		{
			testName:      "Cert Contains Invalid Hostname",
			testIPAddress: correctIPAddress,
			testHostName:  "different-hostname",
			testTime:      time.Now(),
			want:          true,
		},
		{
			testName:      "Cert Will Expire In Less Than Three Days",
			testIPAddress: correctIPAddress,
			testHostName:  correctHostName,
			testTime:      time.Now().AddDate(10, 0, 1), // ~ 48 hours until expiry
			want:          true,
		},
	}
	t.Log("generating test certificate...")
	testCert, err := createTestCert([]net.IP{net.ParseIP(correctIPAddress)}, []string{correctHostName}, testCertExpiryTime)
	assert.Equal(t, nil, err)
	t.Log("successfully generated test certificate")
	for _, c := range testCases {
		t.Run(c.testName, func(t *testing.T) {
			got, err := KubeletCertificateNeedsRegeneration(c.testIPAddress, c.testHostName, testCert, c.testTime)
			assert.Equal(t, nil, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// createTestCert creates a self-signed certificate for use in tests, it incorporates the given ipAddress's, hostNames, and expiry time
func createTestCert(ipAddress []net.IP, hostname []string, notAfter time.Time) (tls.Certificate, error) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2022),
		Subject: pkix.Name{
			Organization:  []string{"Rancher"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Green Pastures"},
			StreetAddress: []string{"123 Cattle Drive"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:    time.Now(),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		SubjectKeyId: []byte{5, 4, 1, 6, 8},
		DNSNames:     hostname,
		IPAddresses:  ipAddress,
	}

	certPriv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &certPriv.PublicKey, privKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPem := new(bytes.Buffer)
	certPrivPem := new(bytes.Buffer)
	if err = pem.Encode(certPem, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return tls.Certificate{}, err
	}
	if err = pem.Encode(certPrivPem, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPriv),
	}); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPem.Bytes(), certPrivPem.Bytes())
}
