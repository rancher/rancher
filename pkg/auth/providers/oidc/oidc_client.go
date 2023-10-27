package oidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

func AddCertKeyToContext(ctx context.Context, certificate, key string) (context.Context, error) {
	if certificate != "" && key != "" {
		certKeyClient := &http.Client{}
		err := GetClientWithCertKey(certKeyClient, certificate, key)
		if err != nil {
			return nil, err
		}
		return oidc.ClientContext(ctx, certKeyClient), nil
	}

	return ctx, nil
}

func GetClientWithCertKey(httpClient *http.Client, certificate, key string) error {
	tlsConfig, err := getTLSClientConfig(certificate, key)
	if err != nil {
		return err
	}

	// Use the default transport to pick up on things like http.ProxyFromEnvironment
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	httpClient.Transport = transport
	return nil
}

func getTLSClientConfig(certificate, key string) (*tls.Config, error) {
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
