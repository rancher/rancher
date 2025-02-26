package ext

import (
	"fmt"
	"sync"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wranglercore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
		logrus.Info("imperative api APIService cert updated")

		caBundle, _ := provider.CurrentCertKeyContent()
		if err := CreateOrUpdateAPIService(apiservice, caBundle); err != nil {
			logrus.WithError(err).Error("failed to update api service")
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
	defer c.secretMu.Unlock()

	c.secret = secret
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
