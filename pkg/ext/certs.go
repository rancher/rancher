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
	"net/http"
	"sync"
	"time"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/dynamiclistener/storage/memory"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/util/keyutil"
	netutils "k8s.io/utils/net"
)

const (
	CertificateBlockType = "CERTIFICATE"

	SecretLabelProvider = "provider"

	certCheckInterval        = time.Hour
	maxRemainingCertLifetime = time.Hour
)

// GenerateSelfSignedCertKey generates a self-signed certificate.
// Based on the self-signed cert generate code in client-go: https://pkg.go.dev/k8s.io/client-go/util/cert#GenerateSelfSignedCertKeyWithFixtures
func GenerateSelfSignedCertKeyWithOpts(host string, expireAfter time.Duration) ([]byte, []byte, error) {
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

var _ dynamiccertificates.SNICertKeyContentProvider = &provider{}
var _ dynamiccertificates.Notifier = &provider{}
var _ dynamiccertificates.CertKeyContentProvider = &provider{}
var _ dynamiclistener.TLSStorage = &provider{}

type provider struct {
	name       string
	secretName string

	sninames []string

	listenerM sync.RWMutex
	listeners []dynamiccertificates.Listener

	storage dynamiclistener.TLSStorage
}

func NewSNIProviderForCname(ctx context.Context, l net.Listener, name string, cnames []string, secrets wranglercorev1.SecretClient) (*provider, net.Listener, http.Handler, error) {
	secretName := fmt.Sprintf("%s-cert-ca", name)
	storage := kubernetes.New(ctx, nil, Namespace, secretName, memory.New())

	provider := &provider{
		name:       name,
		secretName: secretName,

		sninames: cnames,

		storage: storage,
	}

	// todo: we can probably separate this out later
	ca, key, err := kubernetes.LoadOrGenCAChain(secrets, Namespace, provider.secretName)
	if err != nil {
		return nil, nil, nil, err
	}

	ln, h, err := dynamiclistener.NewListenerWithChain(l, storage, ca, key, dynamiclistener.Config{})
	if err != nil {
		return nil, nil, nil, err
	}

	return provider, ln, h, nil
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
	// todo: implement this
	return nil, nil
}

func (p *provider) Get() (*corev1.Secret, error) {
	return p.storage.Get()
}

func (p *provider) Update(s *corev1.Secret) error {
	if err := p.storage.Update(s); err != nil {
		return err
	}

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
