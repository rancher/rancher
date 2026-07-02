package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

// mockResolver satisfies the netResolver interface for testing
type mockResolver struct {
	lookupFunc func(ctx context.Context, host string) ([]string, error)
}

func (m *mockResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return m.lookupFunc(ctx, host)
}

func TestPreStart_Bypass(t *testing.T) {
	t.Setenv("CATTLE_ENTRYPOINT_BYPASS", "true")
	err := preStart(t.Context())
	if err != nil {
		t.Fatalf("expected no error when bypass env is true, got: %v", err)
	}
}

func TestPingCattleServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := t.Context()

	// Test successful ping
	if err := pingCattleServer(ctx, server.URL); err != nil {
		t.Errorf("expected ping to succeed, got: %v", err)
	}

	// Test failing ping status code
	if err := pingCattleServer(ctx, server.URL+"/bad-route"); err == nil {
		t.Error("expected ping to fail for non-200 status code, got nil")
	}
}

func TestPrintResolvedCattleServerHostname(t *testing.T) {
	tc := []struct {
		name          string
		cattleServer  string
		resolver      netResolver
		expectedError bool
	}{
		{
			name:         "domain resolution",
			cattleServer: "https://rancher.my-domain.com:8443",
			resolver: &mockResolver{
				lookupFunc: func(ctx context.Context, host string) ([]string, error) {
					return []string{"192.168.1.50", "192.168.1.51"}, nil
				},
			},
		},
		{
			name:         "IP resolution",
			cattleServer: "https://127.0.0.1:8443",
			resolver: &mockResolver{
				lookupFunc: func(ctx context.Context, host string) ([]string, error) {
					return []string{"127.0.0.1"}, nil
				},
			},
		},
		{
			name:         "Failed resolution",
			cattleServer: "https://does-not-exist.local",
			resolver: &mockResolver{
				lookupFunc: func(ctx context.Context, host string) ([]string, error) {
					return nil, &net.DNSError{Err: "no such host", Name: host}
				},
			},
			expectedError: true,
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			if err := printResolvedCattleServerHostname(t.Context(), tt.cattleServer, tt.resolver); (err != nil) != tt.expectedError {
				t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
			}
		})
	}
}

func sha256Sum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func TestPopulateRancherCACerts_And_Write(t *testing.T) {
	validCertPEM := generateValidPEM(t)
	if validCertPEM[len(validCertPEM)-1] != '\n' {
		validCertPEM = append(validCertPEM, '\n')
	}
	invalidCertPEM := []byte("---BEGIN CERTIFICATE---\nNOT-A-REAL-CERT\n---END CERTIFICATE---\n")

	tc := []struct {
		name          string
		certPayload   []byte
		certChecksum  string
		expectedError string
	}{
		{
			name:         "valid certificate",
			certPayload:  validCertPEM,
			certChecksum: sha256Sum(validCertPEM),
		},
		{
			name:          "invalid certificate",
			certPayload:   invalidCertPEM,
			certChecksum:  sha256Sum(invalidCertPEM),
			expectedError: "does not look like an x509 certificate",
		},
		{
			name:          "integrity failure",
			certPayload:   validCertPEM,
			certChecksum:  "completely-wrong-checksum",
			expectedError: "does not match",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				setting := &v3.Setting{
					Value: string(tt.certPayload),
				}
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(setting); err != nil {
					t.Fatal(err)
					return
				}
			}))
			defer server.Close()
			serverHostname := strings.TrimPrefix(server.URL, "http://")

			tmpDir := t.TempDir()
			mockPaths := certsDirs{
				kubernetesSSLCertsDir: filepath.Join(tmpDir, "k8s-ssl"),
				dockerCertsDir:        filepath.Join(tmpDir, "docker-ssl"),
			}

			if err := populateRancherCACerts(t.Context(), server.URL, tt.certChecksum, mockPaths); err != nil {
				if tt.expectedError == "" {
					t.Errorf("unexpected error: %v", err)
				} else {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			} else if tt.expectedError != "" {
				t.Errorf("expected error matching: %q, got: %v", tt.expectedError, err)
			}

			// Verify file contents were written
			for _, p := range []string{
				filepath.Join(tmpDir, "k8s-ssl/serverca"),
				filepath.Join(tmpDir, fmt.Sprintf("docker-ssl/%s/ca.crt", serverHostname)),
			} {
				if data, err := os.ReadFile(p); err != nil {
					t.Error(err)
				} else if !bytes.Equal(data, tt.certPayload) {
					t.Errorf("kube cert data mismatch")
				}
			}
		})
	}
}
