package ext

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"net"
	"sync"

	"github.com/rancher/dynamiclistener"
	dlc "github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wranglercore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

const (
	CertName = "imperative-api-extension-cert"
)

type listenerFunc func()

func (f listenerFunc) Enqueue() {
	f()
}

func ApiServiceCertListener(provider dynamiccertificates.SNICertKeyContentProvider, apiservice wranglerapiregistrationv1.APIServiceController) dynamiccertificates.Listener {
	return listenerFunc(func() {
		logrus.Info("updating imperative api APIService cert")

		caBundle, _ := provider.CurrentCertKeyContent()
		if err := CreateOrUpdateAPIService(apiservice, caBundle); err != nil {
			logrus.WithError(err).Error("failed to update ipmerative api APIService")
		}
	})
}

var _ dynamiccertificates.SNICertKeyContentProvider = &CertStore{}
var _ dynamiclistener.TLSStorage = &CertStore{}

type CertStore struct {
	listenerMu sync.RWMutex
	listeners  []dynamiccertificates.Listener

	sniNames []string
	name     string

	secretMu sync.RWMutex
	secret   *corev1.Secret
}

func NewStore(name string, sniName []string) *CertStore {
	return &CertStore{
		name:     name,
		sniNames: sniName,

		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CertName,
				Namespace: Namespace,
			},
		},
	}
}

func (c *CertStore) Name() string {
	return c.name
}

func (c *CertStore) AddListener(listener dynamiccertificates.Listener) {
	c.listenerMu.Lock()
	defer c.listenerMu.Unlock()

	c.listeners = append(c.listeners, listener)
}

func (c *CertStore) notify() {
	c.listenerMu.RLock()
	defer c.listenerMu.RUnlock()

	for _, listener := range c.listeners {
		listener.Enqueue()
	}
}

func (c *CertStore) SNINames() []string {
	return c.sniNames
}

func (c *CertStore) CurrentCertKeyContent() ([]byte, []byte) {
	c.secretMu.RLock()
	defer c.secretMu.RUnlock()

	if c.secret == nil {
		return nil, nil
	}

	cert, ok := c.secret.Data[corev1.TLSCertKey]
	if !ok {
		logrus.Errorf("secret does not contain cert field '%s'", corev1.TLSCertKey)
	}

	key, ok := c.secret.Data[corev1.TLSPrivateKeyKey]
	if !ok {
		logrus.Errorf("secret does not contain key field '%s'", corev1.TLSPrivateKeyKey)
	}

	return cert, key
}

func (c *CertStore) Get() (*corev1.Secret, error) {
	c.secretMu.RLock()
	defer c.secretMu.RUnlock()

	if c.secret == nil {
		return nil, fmt.Errorf("no secret found")
	}

	return c.secret, nil
}

func (c *CertStore) Update(secret *corev1.Secret) error {
	c.secretMu.Lock()

	c.secret = secret
	c.secretMu.Unlock()

	c.notify()

	return nil
}

func coreGetterFactory(wranglerCtx *wrangler.Context) func() *wranglercore.Factory {
	return func() *wranglercore.Factory {
		factory, err := core.NewFactoryFromConfig(wranglerCtx.RESTConfig)
		if err != nil {
			panic(fmt.Sprintf("failed to create core factory: %v", err))
		}

		return factory
	}
}

func generateCertKey(cn string) (*x509.Certificate, crypto.Signer, error) {
	signer, err := factory.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate signer: %w", err)
	}

	cert, err := factory.NewSelfSignedCACert(signer, cn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate cert: %w", err)
	}

	return cert, signer, nil
}

func getOrCreateCertKey(secrets wranglercorev1.SecretController, config dynamiclistener.Config) ([]*x509.Certificate, crypto.Signer, error) {
	secret, err := secrets.Get(Namespace, CertName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		cert, key, err := generateCertKey(config.CN)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate cert and key: %w", err)
		}

		return []*x509.Certificate{cert}, key, err
	} else if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret: %w", err)
	}

	cert, err := dlc.ParseCertsPEM(secret.Data[corev1.TLSCertKey])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse cert: %w", err)
	}

	key, err := dlc.ParsePrivateKeyPEM(secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse key: %w", err)
	}

	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("key is not a crypto.Signer: '%T'", key)
	}

	return cert, signer, nil
}

func getListener(wranglerContext *wrangler.Context, store dynamiclistener.TLSStorage) (net.Listener, error) {
	// Only need to listen on localhost because that port will be reached
	// from a remotedialer tunnel on localhost
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp listener: %w", err)
	}

	config := dynamiclistener.Config{
		CN: fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace),
		RegenerateCerts: func() bool {
			_, err := wranglerContext.Core.Secret().Get(Namespace, CertName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		},
	}

	certs, key, err := getOrCreateCertKey(wranglerContext.Core.Secret(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create cert and key: %w", err)
	}

	ln, _, err = dynamiclistener.NewListenerWithChain(ln, store, certs, key, config)
	if err != nil {
		return nil, err
	}

	return ln, nil
}
