package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func HelmClient(secret *corev1.Secret, caBundle []byte, insecureSkipTLSVerify bool) (*http.Client, error) {
	var (
		username  string
		password  string
		tlsConfig tls.Config
	)

	if secret != nil {
		switch secret.Type {
		case corev1.SecretTypeBasicAuth:
			username = string(secret.Data[corev1.BasicAuthUsernameKey])
			password = string(secret.Data[corev1.BasicAuthPasswordKey])
		case corev1.SecretTypeTLS:
			cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		}
	}

	if len(caBundle) > 0 {
		cert, err := x509.ParseCertificate(caBundle)
		if err != nil {
			return nil, err
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		pool.AddCert(cert)
		tlsConfig.RootCAs = pool
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tlsConfig
	transport.TLSClientConfig.InsecureSkipVerify = insecureSkipTLSVerify

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	if username != "" || password != "" {
		client.Transport = &basicRoundTripper{
			username: username,
			password: password,
			next:     client.Transport,
		}
	}

	return client, nil
}

type basicRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (b *basicRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.SetBasicAuth(b.username, b.password)
	return b.next.RoundTrip(request)
}
