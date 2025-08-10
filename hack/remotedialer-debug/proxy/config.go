package proxy

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TLSName         string // certificate client name (SAN)
	CAName          string // certificate authority secret name
	CertCANamespace string // certificate secret namespace
	CertCAName      string // certificate secret name
	Secret          string // remotedialer secret
	ProxyPort       int    // tcp remotedialer-proxy port
	PeerPort        int    // cluster-external service port
	HTTPSPort       int    // https remotedialer-proxy port
}

func requiredString(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", key)
	}
	return value, nil
}

func requiredPort(key string) (int, error) {
	valueStr := os.Getenv(key)
	port, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", key, err)
	}
	if port <= 0 {
		return 0, fmt.Errorf("%s should be greater than 0", key)
	}
	return port, nil
}

func ConfigFromEnvironment() (*Config, error) {
	var err error
	var config Config

	if config.TLSName, err = requiredString("TLS_NAME"); err != nil {
		return nil, err
	}
	if config.CAName, err = requiredString("CA_NAME"); err != nil {
		return nil, err
	}
	if config.CertCANamespace, err = requiredString("CERT_CA_NAMESPACE"); err != nil {
		return nil, err
	}
	if config.CertCAName, err = requiredString("CERT_CA_NAME"); err != nil {
		return nil, err
	}
	if config.Secret, err = requiredString("SECRET"); err != nil {
		return nil, err
	}
	if config.ProxyPort, err = requiredPort("PROXY_PORT"); err != nil {
		return nil, err
	}
	if config.PeerPort, err = requiredPort("PEER_PORT"); err != nil {
		return nil, err
	}
	if config.HTTPSPort, err = requiredPort("HTTPS_PORT"); err != nil {
		return nil, err
	}

	return &config, nil
}
