package oidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rancher/rancher/pkg/features"
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
	if certificate != "" && key != "" {
		keyPair, err := tls.X509KeyPair([]byte(certificate), []byte(key))
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(certificate))
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
				Certificates: []tls.Certificate{keyPair},
			},
		}

		if features.AuthMTLSRespectProxy.Enabled() {
			transport.Proxy = http.ProxyFromEnvironment
		}

		httpClient.Transport = transport
	}
	return nil
}
