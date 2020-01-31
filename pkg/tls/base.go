package tls

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
)

const (
	httpsMode = "https"
	acmeMode  = "acme"
)

var (
	tlsVersions = map[string]uint16{
		"1.2": tls.VersionTLS12,
		"1.1": tls.VersionTLS11,
		"1.0": tls.VersionTLS10,
	}

	// https://golang.org/pkg/crypto/tls/#pkg-constants
	tlsCipherSuites = map[string]uint16{
		"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"TLS_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"TLS_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		"TLS_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}
)

func BaseTLSConfig() (*tls.Config, error) {
	// Get configured minimal tls version
	TLSMinVersion, err := lookupTLSVersion()
	if err != nil {
		return nil, fmt.Errorf("error while configuring minimal TLS version: %s", err)
	}
	// Get configured tls ciphers
	TLSCiphers, err := lookupTLSCiphers()
	if err != nil {
		return nil, fmt.Errorf("error while configuring TLS ciphers: %s", err)
	}

	return &tls.Config{
		PreferServerCipherSuites: true,
		MinVersion:               TLSMinVersion,
		CipherSuites:             TLSCiphers,
	}, nil
}

func lookupTLSVersion() (uint16, error) {
	tlsVersionKeys := getKeysFromMap(tlsVersions)
	settingsTLSMinVersion := settings.TLSMinVersion.Get()
	if val, ok := tlsVersions[settingsTLSMinVersion]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("invalid minimal TLS version [%s], must be one of: %s", settingsTLSMinVersion, strings.Join(tlsVersionKeys, " "))
}

func lookupTLSCiphers() ([]uint16, error) {
	tlsCipherSuitesKeys := getKeysFromMap(tlsCipherSuites)
	settingsTLSCiphers := settings.TLSCiphers.Get()
	sliceTLSCiphers := strings.Split(settingsTLSCiphers, ",")

	var TLSCiphers []uint16
	for _, TLSCipher := range sliceTLSCiphers {
		val, ok := tlsCipherSuites[strings.TrimSpace(TLSCipher)]
		if !ok {
			return []uint16{}, fmt.Errorf("unsupported cipher [%s], must be one or more from: %s", TLSCipher, strings.Join(tlsCipherSuitesKeys, " "))
		}
		TLSCiphers = append(TLSCiphers, val)
	}
	return TLSCiphers, nil
}

func getKeysFromMap(input map[string]uint16) []string {
	var keys []string
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}
