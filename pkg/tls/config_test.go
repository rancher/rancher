package tls

import (
	"crypto/tls"
	"reflect"
	"testing"
)

func TestBaseTLSConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		minVersion    string
		ciphers       string
		cfg           *tls.Config
		errorExpected bool
	}{
		{
			name:       "valid base config for TLS 1.0",
			minVersion: "1.0",
			ciphers:    "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			cfg: &tls.Config{
				MinVersion:   tls.VersionTLS10,
				CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256},
			},
		},
		{
			name:       "valid base config for TLS 1.1",
			minVersion: "1.1",
			ciphers:    "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			cfg: &tls.Config{
				MinVersion:   tls.VersionTLS11,
				CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256},
			},
		},
		{
			name:       "valid base config for TLS 1.2 with ciphers from 1.2 and 1.3",
			minVersion: "1.2",
			ciphers:    "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_AES_128_GCM_SHA256",
			cfg: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_AES_128_GCM_SHA256},
			},
		},
		{
			name:       "valid base config for TLS 1.3",
			minVersion: "1.3",
			ciphers:    "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384",
			cfg: &tls.Config{
				MinVersion:   tls.VersionTLS13,
				CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_AES_256_GCM_SHA384},
			},
		},
		{
			name:          "unsupported min version",
			minVersion:    "3.4",
			ciphers:       "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			errorExpected: true,
		},
		{
			name:          "unknown cipher in cipher suite",
			minVersion:    "1.3",
			ciphers:       "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,BAD_CIPHER",
			errorExpected: true,
		},
		{
			name:          "unsupported min version and unknown cipher in cipher suite",
			minVersion:    "3.4",
			ciphers:       "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,BAD_CIPHER",
			errorExpected: true,
		},
		{
			name:          "wrong cipher for TLS 1.3",
			minVersion:    "1.3",
			ciphers:       "TLS_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			errorExpected: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := baseTLSConfig(test.minVersion, test.ciphers)
			if err != nil && !test.errorExpected {
				t.Fatalf("got an unexpected error: %v", err)
			}
			if err == nil && test.errorExpected {
				t.Fatalf("expected an error but did not get it")
			}
			if !reflect.DeepEqual(got, test.cfg) {
				t.Errorf("\nexpected\n%v\ngot\n%v", test.cfg, got)
			}
		})
	}
}
