package ext

import (
	"context"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	netutils "k8s.io/utils/net"
)

const (
	certCheckInterval = time.Hour
)

// GenerateSelfSignedCertKey generates a self-signed certificate.
// Based on the self-signed cert generate code in client-go: https://pkg.go.dev/k8s.io/client-go/util/cert#GenerateSelfSignedCertKeyWithFixtures
func GenerateSelfSignedCertKeyWithOpts(host string, expireAfter time.Duration) (*x509.Certificate, crypto.Signer, error) {
	// valid for an extra check interval before current time to ensure total cert coverage and avoid any issues with clock skew
	validFrom := time.Now().Add(-certCheckInterval)
	maxAge := expireAfter

	caKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, err
	}
	serial = new(big.Int).Add(serial, big.NewInt(1))
	caTemplate := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s-ca@%d", host, time.Now().Unix()),
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	caCertificate, err := x509.ParseCertificate(caDERBytes)
	if err != nil {
		return nil, nil, err
	}

	priv, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// returns a uniform random value in [0, max-1), then add 1 to serial to make it a uniform random value in [1, max).
	serial, err = cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64-1))
	if err != nil {
		return nil, nil, err
	}
	serial = new(big.Int).Add(serial, big.NewInt(1))
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s@%d", host, time.Now().Unix()),
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := netutils.ParseIPSloppy(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(cryptorand.Reader, &template, caCertificate, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	ca, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, err
	}

	return ca, priv, nil
}

var _ dynamiccertificates.SNICertKeyContentProvider = &provider{}
var _ dynamiccertificates.Notifier = &provider{}
var _ dynamiccertificates.CertKeyContentProvider = &provider{}
var _ dynamiclistener.TLSStorage = &provider{}

type provider struct {
	name       string
	secretName string

	// todo: this only needs to be 1 long
	sninames []string

	listenerM sync.RWMutex
	listeners []dynamiccertificates.Listener

	secretMu sync.RWMutex
	secret   *corev1.Secret
}

func newCertProvider(ctx context.Context, wranglerCtx wrangler.Context, l net.Listener, name string, cnames []string) (*provider, error) {
	secretName := fmt.Sprintf("%s-ca", name)

	provider := &provider{
		name:       name,
		secretName: secretName,

		sninames: cnames,
	}

	return provider, nil
}

func (p *provider) AddListener(listener dynamiccertificates.Listener) {
	p.listenerM.Lock()
	defer p.listenerM.Unlock()

	p.listeners = append(p.listeners, listener)
}

func (p *provider) notify() {
	p.listenerM.RLock()
	defer p.listenerM.RUnlock()

	for _, listener := range p.listeners {
		listener.Enqueue()
	}
}

func (p *provider) SNINames() []string {
	return p.sninames
}

func (p *provider) Name() string {
	return p.name
}

func (p *provider) CurrentCertKeyContent() ([]byte, []byte) {
	secret, err := p.Get()
	if err != nil {
		logrus.Errorf("could not get imperative api cert content: %w", err)
		return nil, nil
	}

	if secret == nil {
		return nil, nil
	}

	return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey]
}

func (p *provider) Get() (*corev1.Secret, error) {
	p.secretMu.RLock()
	defer p.secretMu.RUnlock()

	return p.secret, nil
}

func (p *provider) Update(s *corev1.Secret) error {
	p.secretMu.Lock()
	p.secret = s
	p.secretMu.Unlock()

	p.notify()

	return nil
}

type listenerFunc func()

func (f listenerFunc) Enqueue() {
	f()
}

func ApiServiceCertListener(provider dynamiccertificates.SNICertKeyContentProvider, apiservice wranglerapiregistrationv1.APIServiceController) dynamiccertificates.Listener {
	return listenerFunc(func() {
		logrus.Info("imperative api APIService cert updated")

		caBundle, _ := provider.CurrentCertKeyContent()
		if err := CreateOrUpdateAPIService(apiservice, caBundle); err != nil {
			logrus.WithError(err).Error("failed to update api service")
		}
	})
}

func getListener(ctx context.Context, secrets wranglercorev1.SecretController, p *provider, tcp net.Listener) (net.Listener, http.Handler, error) {
	storage := kubernetes.Load(ctx, secrets, Namespace, p.secretName, p)

	ca, signer, err := GenerateSelfSignedCertKeyWithOpts(p.sninames[0], time.Hour*24*30*3)
	if err != nil {
		return nil, nil, fmt.Errorf("gailed to generate initial certs: %w", err)
	}

	ln, h, err := dynamiclistener.NewListenerWithChain(tcp, storage, []*x509.Certificate{ca}, signer, dynamiclistener.Config{
		CN: p.sninames[0],
		// Organization:          []string{},
		// TLSConfig: &tls.Config{},
		// SANs:                  []string{},
		// MaxSANs:               0,
		// ExpirationDaysCheck:   0,
		// CloseConnOnCertChange: false,
		RegenerateCerts: func() bool {
			return true
		},
		FilterCN: func(...string) []string {
			return []string{p.sninames[0]}
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return ln, h, nil
}
