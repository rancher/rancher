package http

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func HelmClient(secret *corev1.Secret, caBundle []byte, insecureSkipTLSVerify bool, disableSameOriginCheck bool, repoURL string) (*http.Client, error) {
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
			username:               username,
			password:               password,
			disableSameOriginCheck: disableSameOriginCheck,
			repoURL:                repoURL,
			next:                   client.Transport,
		}
	}

	return client, nil
}

type basicRoundTripper struct {
	username               string
	password               string
	disableSameOriginCheck bool
	repoURL                string
	next                   http.RoundTripper
}

func (b *basicRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	attachHeader, err := shouldAttachBasicAuthHeader(b.repoURL, b.disableSameOriginCheck, request)
	if err != nil {
		return nil, err
	}
	if attachHeader {
		request.SetBasicAuth(b.username, b.password)
	}
	return b.next.RoundTrip(request)
}

// Return bool for if the Basic Auth Header should be attached to the request
func shouldAttachBasicAuthHeader(repoURL string, disableSameOriginCheck bool, request *http.Request) (bool, error) {
	if disableSameOriginCheck {
		return true, nil
	}
	parsedRepoURL, err := url.Parse(repoURL)
	if err != nil {
		return false, err
	}
	// check to see if request is being made to the same domain or subdomain as the repo url
	// to determine if it is the same origin, if it is not, then do not attach the auth header
	return isDomainOrSubdomain(request.URL, parsedRepoURL), nil
}

// Using isDomainOrSubdomain from http.client go library
// this is to ensure that we are using as close to the upstream as possible for same origin checks
func isDomainOrSubdomain(reqURL, repoURL *url.URL) bool {
	parent := canonicalAddr(repoURL)
	sub := canonicalAddr(reqURL)
	if sub == parent {
		return true
	}
	// If sub is "foo.example.com" and parent is "example.com",
	// that means sub must end in "."+parent.
	// Do it without allocating.
	if !strings.HasSuffix(sub, parent) {
		return false
	}
	return sub[len(sub)-len(parent)-1] == '.'
}

// Using canonicalAddr from http.client go library
// canonicalAddr returns url.Host but always with a ":port" suffix
// the idnaASCII checks have been removed
func canonicalAddr(url *url.URL) string {
	var portMap = map[string]string{
		"http":   "80",
		"https":  "443",
		"socks5": "1080",
	}
	addr := url.Hostname()
	port := url.Port()
	if port == "" {
		port = portMap[url.Scheme]
	}
	return net.JoinHostPort(addr, port)
}
