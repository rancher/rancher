package oidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

func (o *OpenIDCProvider) AddCertKeyToContext(ctx *context.Context, certificate, key string) error {
	if certificate != "" && key != "" {
		var certKeyClient http.Client
		err := GetClientWithCertKey(&certKeyClient, certificate, key)
		if err != nil {
			return err
		}
		oidc.ClientContext(*ctx, &certKeyClient)
	}
	return nil
}

func GetClientWithCertKey(httpClient *http.Client, certificate, key string) error {
	if certificate != "" && key != "" {
		keyPair, err := tls.X509KeyPair([]byte(certificate), []byte(key))
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(certificate))
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
				Certificates: []tls.Certificate{keyPair},
			},
		}
	}
	return nil
}
