package oidc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

func createCert(isCA bool) (*x509.Certificate, *rsa.PrivateKey, error) {
	serialNo, err := rand.Int(rand.Reader, big.NewInt(int64(time.Now().Year())))
	if err != nil {
		return nil, nil, err
	}

	keyUsage := x509.KeyUsageDigitalSignature
	if isCA {
		keyUsage = keyUsage | x509.KeyUsageCertSign
	}

	cert := &x509.Certificate{
		SerialNumber: serialNo,
		Subject: pkix.Name{
			Organization:  []string{fmt.Sprintf("Rancher - %d", serialNo)},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Green Pastures"},
			StreetAddress: []string{"123 Cattle Drive"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(5 * time.Minute),
		KeyUsage:    keyUsage,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IsCA:        isCA,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func getPEMBytes(cert []byte, key *rsa.PrivateKey) ([]byte, []byte) {
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})

	keyPEM := new(bytes.Buffer)
	pem.Encode(keyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return certPEM.Bytes(), keyPEM.Bytes()
}

func createClientCert(ca *x509.Certificate, rootKey *rsa.PrivateKey) ([]byte, []byte, error) {
	cert, key, err := createCert(false)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM, keyPEM := getPEMBytes(certBytes, key)
	return certPEM, keyPEM, nil
}

func TestAddCertKeyToContext(t *testing.T) {
	rootCA, rootKey, err := createCert(true)
	if err != nil {
		t.Fatalf("unable to create test CA Key: %s", err)
	}

	rootCABytes, err := x509.CreateCertificate(rand.Reader, rootCA, rootCA, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("unable to parse generated CA certs")
	}

	_, rootKeyPem := getPEMBytes(rootCABytes, rootKey)

	clientCertBytes, clientKeyBytes, err := createClientCert(rootCA, rootKey)
	if err != nil {
		t.Fatalf("unable to generate test client cert")
	}

	pool, _ := x509.SystemCertPool()
	poolWithCert, _ := x509.SystemCertPool()
	cert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	if err != nil {
		t.Fatalf("unable to parse generated certs")
	}
	poolWithCert.AppendCertsFromPEM(clientCertBytes)

	testCases := []struct {
		name       string
		cert       string
		key        string
		shouldFail bool
		wantPool   *x509.CertPool
		wantCerts  []tls.Certificate
	}{
		{
			name:       "no cert or key",
			cert:       "",
			key:        "",
			shouldFail: false,
			wantPool:   pool,
			wantCerts:  nil,
		},
		{
			name:       "valid cert and key",
			cert:       string(clientCertBytes),
			key:        string(clientKeyBytes),
			shouldFail: false,
			wantPool:   poolWithCert,
			wantCerts:  []tls.Certificate{cert},
		},
		{
			name:       "valid cert with no key",
			cert:       string(clientCertBytes),
			key:        "",
			shouldFail: false,
			wantPool:   pool,
			wantCerts:  nil,
		},
		{
			name:       "no cert with valid key",
			cert:       "",
			key:        string(clientKeyBytes),
			shouldFail: false,
			wantPool:   pool,
			wantCerts:  nil,
		},
		{
			name:       "mismatched cert and key",
			cert:       string(clientCertBytes),
			key:        string(rootKeyPem),
			shouldFail: true,
			wantPool:   nil,
			wantCerts:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := AddCertKeyToContext(context.Background(), tc.cert, tc.key)
			assert.Equal(t, err != nil, tc.shouldFail)
			if tc.shouldFail && err != nil {
				return
			}

			got, ok := ctx.Value(oauth2.HTTPClient).(*http.Client)
			require.True(t, ok, "expected to find an http client accessible in the context but didn't")

			gotTransport := got.Transport.(*http.Transport)
			if !gotTransport.TLSClientConfig.RootCAs.Equal(tc.wantPool) {
				t.Fatalf("system cert pool did not match desired")
			}

			assert.Equal(t, tc.wantCerts, gotTransport.TLSClientConfig.Certificates, "cert did not match desired")
		})
	}
}
