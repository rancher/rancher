package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func Test_RootCATransport(t *testing.T) {
	tmpDir := t.TempDir()

	validCertPath := filepath.Join(tmpDir, "valid.pem")
	_ = os.WriteFile(validCertPath, generateValidPEM(t), 0644)

	// Create a corrupt PEM file
	corruptPath := filepath.Join(tmpDir, "corrupt.pem")
	_ = os.WriteFile(corruptPath, []byte("this is not a certificate"), 0644)

	tests := []struct {
		name           string
		caFileLocation string
		envProxy       string
		expectNil      bool
		checkProxyHost string
	}{
		{
			name:           "Valid CA file",
			caFileLocation: validCertPath,
			expectNil:      false,
		},
		{
			name:           "Missing CA File",
			caFileLocation: filepath.Join(tmpDir, "nonexistent.pem"),
			expectNil:      true,
		},
		{
			name:           "Corrupt CA file",
			caFileLocation: corruptPath,
			expectNil:      true,
		},
		{
			name:           "Respects proxy env with valid CA file",
			caFileLocation: validCertPath,
			envProxy:       "http://proxy.internal:8080",
			expectNil:      false,
			checkProxyHost: "proxy.internal:8080",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envProxy != "" {
				t.Setenv("HTTPS_PROXY", tc.envProxy)
			}

			transport := rootCATransport(tc.caFileLocation)
			if tc.expectNil {
				if transport != nil {
					t.Errorf("expected nil transport for case %s", tc.name)
				}
				return
			}

			if transport == nil {
				t.Fatalf("expected non-nil transport for case %s", tc.name)
			}

			if tc.checkProxyHost != "" {
				req, _ := http.NewRequest("GET", "https://google.com", nil)
				pURL, _ := transport.Proxy(req)
				if pURL == nil || pURL.Host != tc.checkProxyHost {
					t.Errorf("expected proxy host %s, got %v", tc.checkProxyHost, pURL)
				}
			}
		})
	}
}

func generateValidPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}
