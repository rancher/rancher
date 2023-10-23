package oidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

func AddCertKeyToContext(ctx context.Context, certificate, key string) (context.Context, error) {
	certKeyClient, err := GetClientWithCertKey(certificate, key)
	if err != nil {
		return nil, err
	}
	return oidc.ClientContext(ctx, certKeyClient), nil
}

func GetClientWithCertKey(certificate, key string) (*http.Client, error) {
	// Use the default transport to pick up on things like http.ProxyFromEnvironment
	transport := http.DefaultTransport.(*http.Transport).Clone()

	tlsConfig, err := getTLSClientConfig(certificate, key)
	if err != nil {
		return &http.Client{Transport: transport}, err
	}

	transport.TLSClientConfig = tlsConfig
	return &http.Client{Transport: transport}, nil
}

func getTLSClientConfig(certificate, key string) (*tls.Config, error) {
	if certificate == "" || key == "" {
		return nil, errors.New("certificate or key provided, but not both")
	}

	keyPair, err := tls.X509KeyPair([]byte(certificate), []byte(key))
	if err != nil {
		return nil, err
	}

	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{keyPair},
	}, nil
}
