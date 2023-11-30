package oidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

func getClientCertificates(certificate, key string) ([]tls.Certificate, error) {
	cert, err := tls.X509KeyPair([]byte(certificate), []byte(key))
	if err != nil {
		return nil, fmt.Errorf("could not parse cert/key pair: %w", err)
	}

	return []tls.Certificate{cert}, nil
}

func getHTTPClient(certificate, key string) (*http.Client, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if certificate != "" && key != "" {
		certs, err := getClientCertificates(certificate, key)
		if err != nil {
			return nil, err
		}

		pool.AppendCertsFromPEM([]byte(certificate))
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:      pool,
					Certificates: certs,
				},
			},
		}, nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.RootCAs = pool
	return &http.Client{
		Transport: transport,
	}, nil
}

func AddCertKeyToContext(ctx context.Context, certificate, key string) (context.Context, error) {
	client, err := getHTTPClient(certificate, key)
	if err != nil {
		return nil, err
	}

	return oidc.ClientContext(ctx, client), nil
}
