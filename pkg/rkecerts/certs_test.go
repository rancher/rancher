package rkecerts

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/rancher/rke/pki"
	"github.com/stretchr/testify/assert"
)

func TestBundle_SafeMarshal(t *testing.T) {

	rsaKey, err := rsa.GenerateKey(rand.Reader, 12)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		certs   map[string]pki.CertificatePKI
		want    string
		wantErr bool
	}{
		{"base",
			map[string]pki.CertificatePKI{

				pki.CACertName: {
					Key:        rsaKey,
					KeyPEM:     `PRETENDBASE64_TOBEREMOVED=`,
					Name:       "test",
					KeyEnvName: "TEST_ENV",
					KeyPath:    "test",
				}},
			`{"kube-ca":{"certificatePEM":"","keyPEM":"","config":"","name":"test","commonName":"","ouName":"","envName":"","path":"","keyEnvName":"TEST_ENV","keyPath":"test","configEnvName":"","configPath":"","CertPEM":"","KeyPEM":""}}` + "\n",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bundle{
				certs: tt.certs,
			}
			got, err := b.SafeMarshal()
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeMarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
