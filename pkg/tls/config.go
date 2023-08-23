package tls

import (
	"crypto/tls"
	"fmt"
	"sort"
	"strings"
)

var (
	validVersions = map[string]uint16{
		"1.3": tls.VersionTLS13,
		"1.2": tls.VersionTLS12,
		// Deprecated.
		"1.1": tls.VersionTLS11,
		// Deprecated.
		"1.0": tls.VersionTLS10,
	}

	// https://golang.org/pkg/crypto/tls/#pkg-constants
	validCiphers10to12 = map[string]uint16{
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

	validCiphers13 = map[string]uint16{
		"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
		"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
		"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,
	}
)

func baseTLSConfig(minVersion, ciphers string) (*tls.Config, error) {
	version, err := validatedMinVersion(minVersion)
	if err != nil {
		return nil, err
	}
	cipherSuites, err := validatedCiphers(ciphers, version)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   version,
		CipherSuites: cipherSuites,
	}, nil
}

func validatedMinVersion(version string) (uint16, error) {
	if val, ok := validVersions[strings.TrimSpace(version)]; ok {
		return val, nil
	}
	valid := keysFromMap(validVersions)
	sort.Strings(valid)
	return 0, fmt.Errorf("unsupported minimal TLS version %s, must be one of: %s", version, strings.Join(valid, ", "))
}

func validatedCiphers(ciphers string, version uint16) ([]uint16, error) {
	split := strings.Split(ciphers, ",")
	if version == tls.VersionTLS13 {
		return validatedCipherSet(split, validCiphers13)
	}
	return validatedCipherSet(split, union(validCiphers10to12, validCiphers13))
}

func validatedCipherSet(ciphers []string, validCiphers map[string]uint16) ([]uint16, error) {
	tlsCiphers := make([]uint16, 0, len(ciphers))
	for _, cipher := range ciphers {
		c, ok := validCiphers[strings.TrimSpace(cipher)]
		if !ok {
			valid := keysFromMap(validCiphers)
			sort.Strings(valid)
			return nil, fmt.Errorf("unsupported cipher %s, must be one or more of: %s", cipher, strings.Join(valid, "\n"))
		}
		tlsCiphers = append(tlsCiphers, c)
	}
	return tlsCiphers, nil
}

func union(cipherSets ...map[string]uint16) map[string]uint16 {
	result := make(map[string]uint16)
	for _, set := range cipherSets {
		for k, v := range set {
			result[k] = v
		}
	}
	return result
}

func keysFromMap(input map[string]uint16) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}
