package ext

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/util/keyutil"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

var _ dynamiccertificates.SNICertKeyContentProvider = &rotatingSNIProvider{}
var _ dynamiccertificates.Notifier = &rotatingSNIProvider{}
var _ dynamiccertificates.CertKeyContentProvider = &rotatingSNIProvider{}

type rotatingSNIProvider struct {
	name       string
	secretName string

	sninames []string

	listenerM sync.RWMutex
	listeners []dynamiccertificates.Listener

	expiresAfter time.Duration

	contentMu sync.RWMutex
	cert      []byte
	key       []byte

	secrets wranglercorev1.SecretController
}

func NewSNIProviderForCname(name string, cnames []string, secrets wranglercorev1.SecretController) (*rotatingSNIProvider, error) {
	content := &rotatingSNIProvider{
		name:       name,
		secretName: fmt.Sprintf("%s-cert-ca", name),

		sninames: cnames,

		expiresAfter: time.Hour * 24 * 90,

		secrets: secrets,
	}

	return content, nil
}

func (p *rotatingSNIProvider) AddListener(listener dynamiccertificates.Listener) {
	p.listenerM.Lock()
	defer p.listenerM.Unlock()

	p.listeners = append(p.listeners, listener)
}

func (p *rotatingSNIProvider) notify() {
	p.listenerM.RLock()
	defer p.listenerM.RUnlock()

	for _, listener := range p.listeners {
		listener.Enqueue()
	}
}

func (p *rotatingSNIProvider) SNINames() []string {
	return p.sninames
}

func (p *rotatingSNIProvider) Name() string {
	return p.name
}

func (p *rotatingSNIProvider) CurrentCertKeyContent() ([]byte, []byte) {
	p.contentMu.RLock()
	defer p.contentMu.RUnlock()

	return bytes.Clone(p.cert), bytes.Clone(p.key)
}

func (p *rotatingSNIProvider) Run(stopChan <-chan struct{}) error {
	logrus.Info("starting imperative api cert rotator")

	req, err := labels.NewRequirement(SecretLabelProvider, selection.Equals, []string{p.name})
	if err != nil {
		return fmt.Errorf("failed to create label requirement: %w", err)
	}

	watcher, err := p.secrets.Watch(Namespace, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*req).String(),
	})
	if err != nil {
		return fmt.Errorf("failed to create secret watcher: %w", err)
	}

	if err := p.handleCert(); err != nil {
		logrus.Error(err)
	}

	for {
		select {
		case <-stopChan:
			logrus.Info("stopping imperative api cert rotator")

			watcher.Stop()

			if err := p.secrets.Delete(Namespace, p.secretName, &metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
				logrus.Error(err)
			}

			return nil
		case <-time.After(certCheckInterval):
			if err := p.handleCert(); err != nil {
				logrus.Error(err)
			}
		case event := <-watcher.ResultChan():
			if err := p.handleCertEvent(event); err != nil {
				logrus.Error(err)
			}
		}
	}
}

func (p *rotatingSNIProvider) createOrUpdateCerts(secret *corev1.Secret) error {
	cert, key, err := GenerateSelfSignedCertKeyWithOpts(p.sninames[0], p.expiresAfter)
	if err != nil {
		return fmt.Errorf("failed to generate cert: %w", err)
	}

	if secret == nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.secretName,
				Namespace: Namespace,
				Labels: map[string]string{
					SecretLabelProvider: p.name,
				},
			},
			Data: map[string][]byte{
				corev1.TLSPrivateKeyKey: key,
				corev1.TLSCertKey:       cert,
			},
		}

		if _, err := p.secrets.Create(secret); err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}

		logrus.Info("created imperative api cert secret")
	} else {
		secret.Data[corev1.TLSCertKey] = cert
		secret.Data[corev1.TLSPrivateKeyKey] = key

		if _, err := p.secrets.Update(secret); err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}

		logrus.Info("updated imperative api cert secret")
	}

	p.contentMu.Lock()
	p.cert = cert
	p.key = key
	p.contentMu.Unlock()

	p.notify()

	return nil
}

func (p *rotatingSNIProvider) willCertExpireWithinDuration(secret *corev1.Secret, d time.Duration) (bool, error) {
	certData, ok := secret.Data[corev1.TLSCertKey]
	if !ok {
		return false, fmt.Errorf("secret does not contain field '%s'", corev1.TLSCertKey)
	}

	keyData, ok := secret.Data[corev1.TLSPrivateKeyKey]
	if !ok {
		return false, fmt.Errorf("secret does not contain field '%s'", corev1.TLSPrivateKeyKey)
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return false, fmt.Errorf("failed to parse cert: %w", err)
	}

	if time.Now().Add(d).After(cert.Leaf.NotAfter) {
		return true, nil
	}

	return false, nil
}

func (p *rotatingSNIProvider) handleCert() error {
	secret, err := p.secrets.Get(Namespace, p.secretName, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		if err := p.createOrUpdateCerts(nil); err != nil {
			return fmt.Errorf("failed to create new cert: %w", err)
		}

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	willExpire, err := p.willCertExpireWithinDuration(secret, maxRemainingCertLifetime)
	if err != nil {
		return fmt.Errorf("failed to check if cert will expire: %w", err)
	}

	if willExpire {
		logrus.Infof("imperative api cabundle will expire within '%s', regenerating", maxRemainingCertLifetime)
		if err := p.createOrUpdateCerts(secret); err != nil {
			return fmt.Errorf("failed to update expired cert: %w", err)
		}
	}

	return nil
}

func (p *rotatingSNIProvider) handleCertEvent(event watch.Event) error {
	switch event.Type {
	case watch.Added, watch.Modified:
		secret := event.Object.(*corev1.Secret)
		certData, ok := secret.Data[corev1.TLSCertKey]
		if !ok {
			return fmt.Errorf("secret does not contain field '%s'", corev1.TLSCertKey)
		}

		keyData, ok := secret.Data[corev1.TLSPrivateKeyKey]
		if !ok {
			return fmt.Errorf("secret does not contain field '%s'", corev1.TLSPrivateKeyKey)
		}

		p.contentMu.Lock()
		p.cert = certData
		p.key = keyData
		p.contentMu.Unlock()

		p.notify()
	case watch.Deleted:
		if err := p.createOrUpdateCerts(nil); err != nil {
			return err
		}
	}

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
