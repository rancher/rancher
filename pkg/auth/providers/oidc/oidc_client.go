package oidc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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

func FetchAuthURL(config map[string]interface{}) (string, error) {
	// If the authEndpoint is already configured, use that
	if authURL, ok := config["authEndpoint"]; ok {
		return authURL.(string), nil
	}

	issuerURL, ok := config["issuer"].(string)
	if !ok {
		return "", fmt.Errorf("both authEndpoint and issuerURL are missing in the authConfig")
	}

	discoveryURL := fmt.Sprintf("%s/.well-known/openid-configuration", issuerURL)
	resp, err := http.Get(discoveryURL)
	if err != nil {
		return "", fmt.Errorf("unable to fetch discovery information for OIDC provider: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch discovery document: %s", resp.Status)
	}

	var discoveryInfo struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discoveryInfo); err != nil {
		return "", fmt.Errorf("unable to decode the OIDC discovery response %v", err)
	}

	return discoveryInfo.AuthorizationEndpoint, nil
}
