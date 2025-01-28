package ext

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net"
	"sync"
	"time"

	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/util/keyutil"
	netutils "k8s.io/utils/net"
)

const (
	CertificateBlockType = "CERTIFICATE"
)

func ipsToStrings(ips []net.IP) []string {
	ss := make([]string, 0, len(ips))
	for _, ip := range ips {
		ss = append(ss, ip.String())
	}
	return ss
}

// GenerateSelfSignedCertKey generates a self-signed certificate.
// Based on the self-signed cert generate code in client-go: https://pkg.go.dev/k8s.io/client-go/util/cert#GenerateSelfSignedCertKeyWithFixtures
func GenerateSelfSignedCertKeyWithOpts(host string, expireAfter time.Duration) ([]byte, []byte, error) {
	validFrom := time.Now().Add(-time.Hour) // valid an hour earlier to avoid flakes due to clock skew
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

	// Generate cert, followed by ca
	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: CertificateBlockType, Bytes: derBytes}); err != nil {
		return nil, nil, err
	}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: CertificateBlockType, Bytes: caDERBytes}); err != nil {
		return nil, nil, err
	}

	// Generate key
	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(&keyBuffer, &pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBuffer.Bytes(), nil
}

var _ dynamiccertificates.SNICertKeyContentProvider = &rotatingSNIProvider{}
var _ dynamiccertificates.Notifier = &rotatingSNIProvider{}
var _ dynamiccertificates.CertKeyContentProvider = &rotatingSNIProvider{}

type rotatingSNIProvider struct {
	name     string
	sninames []string

	listeners []dynamiccertificates.Listener

	expiresAfter time.Duration

	certMu sync.RWMutex
	cert   []byte
	key    []byte
}

func NewSNIProviderForCname(name string, cnames []string) (*rotatingSNIProvider, error) {
	content := &rotatingSNIProvider{
		name:         name,
		sninames:     cnames,
		expiresAfter: time.Hour * 24 * 365,
	}

	return content, nil
}

func (p *rotatingSNIProvider) AddListener(listener dynamiccertificates.Listener) {
	p.listeners = append(p.listeners, listener)
}

func (p *rotatingSNIProvider) SNINames() []string {
	return p.sninames
}

func (p *rotatingSNIProvider) Name() string {
	return p.name
}

func (p *rotatingSNIProvider) CurrentCertKeyContent() ([]byte, []byte) {
	p.certMu.RLock()
	defer p.certMu.RUnlock()

	return bytes.Clone(p.cert), bytes.Clone(p.key)
}

func (p *rotatingSNIProvider) Run(ctx context.Context) {
	logrus.Info("starting extension api cert rotator")
	defer logrus.Info("stopping extension api cert rotator")

	if err := p.updateCerts(); err != nil {
		logrus.WithError(err).Errorf("initial cert rotation failed")
	}

	go wait.Until(func() {
		if err := p.updateCerts(); err != nil {
			logrus.WithError(err).Error("failed to update extension api cert")
		}
	}, p.expiresAfter, ctx.Done())

	<-ctx.Done()
}

func (p *rotatingSNIProvider) updateCerts() error {
	logrus.Info("updating extension api cert")

	cert, key, err := GenerateSelfSignedCertKeyWithOpts("extension-api", p.expiresAfter)
	if err != nil {
		return fmt.Errorf("failed to generate self signed cert: %w", err)
	}

	p.certMu.Lock()
	p.cert = cert
	p.key = key
	p.certMu.Unlock()

	for _, listener := range p.listeners {
		listener.Enqueue()
	}

	return nil
}

type listenerFunc func()

func (f listenerFunc) Enqueue() {
	f()
}

func ApiServiceCertListener(provider dynamiccertificates.SNICertKeyContentProvider, apiservice wranglerapiregistrationv1.APIServiceController) dynamiccertificates.Listener {
	return listenerFunc(func() {
		logrus.Info("api service cert updated")
		caBundle, _ := provider.CurrentCertKeyContent()
		if err := CreateOrUpdateAPIService(apiservice, caBundle); err != nil {
			logrus.WithError(err).Error("failed to update api service")
		}
	})
}
