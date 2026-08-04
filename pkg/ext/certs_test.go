package ext

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
)

type secretControllerFake struct {
	mu sync.RWMutex

	secrets map[string]*corev1.Secret

	createCount bool
	updateCount bool
	deleteCount bool
}

var _ wranglercorev1.SecretClient = (*secretControllerFake)(nil)

func newSecretControllerFake() *secretControllerFake {
	return &secretControllerFake{secrets: map[string]*corev1.Secret{}}
}

func (f *secretControllerFake) key(namespace, name string) string {
	return namespace + "/" + name
}

func (f *secretControllerFake) stored(namespace, name string) (*corev1.Secret, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	secret, ok := f.secrets[f.key(namespace, name)]
	if !ok || secret == nil {
		return nil, false
	}

	return secret.DeepCopy(), true
}

func (f *secretControllerFake) set(secret *corev1.Secret) *corev1.Secret {
	if secret == nil {
		return nil
	}

	copy := secret.DeepCopy()
	f.secrets[f.key(copy.Namespace, copy.Name)] = copy
	return copy.DeepCopy()
}

func (f *secretControllerFake) Create(secret *corev1.Secret) (*corev1.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.secrets[f.key(secret.Namespace, secret.Name)]; ok {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{}, secret.Name)
	}

	f.createCount = true
	return f.set(secret), nil
}

func (f *secretControllerFake) Update(secret *corev1.Secret) (*corev1.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.secrets[f.key(secret.Namespace, secret.Name)]; !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, secret.Name)
	}

	f.updateCount = true
	return f.set(secret), nil
}

func (f *secretControllerFake) UpdateStatus(secret *corev1.Secret) (*corev1.Secret, error) {
	return f.Update(secret)
}

func (f *secretControllerFake) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.secrets[f.key(namespace, name)]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{}, name)
	}

	f.deleteCount = true
	delete(f.secrets, f.key(namespace, name))
	return nil
}

func (f *secretControllerFake) Get(namespace, name string, options metav1.GetOptions) (*corev1.Secret, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	secret, ok := f.secrets[f.key(namespace, name)]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	}

	return secret.DeepCopy(), nil
}

func (f *secretControllerFake) List(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error) {
	return &corev1.SecretList{}, nil
}

func (f *secretControllerFake) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

func (f *secretControllerFake) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (*corev1.Secret, error) {
	return nil, nil
}

func (f *secretControllerFake) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.ClientInterface[*corev1.Secret, *corev1.SecretList], error) {
	return f, nil
}

func (f *secretControllerFake) DeleteCollection(namespace string, deleteOpts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return nil
}

func parseLeafCertificate(t *testing.T, certBytes []byte) *x509.Certificate {
	t.Helper()

	block, rest := pem.Decode(certBytes)
	if assert.NotNil(t, block) {
		assert.Equal(t, CertificateBlockType, block.Type)
	}
	assert.NotEmpty(t, rest)

	parsed, err := x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)
	return parsed
}

func TestGenerateSelfSignedCertKeyWithOpts(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		expireAfter time.Duration
		wantIP      bool
	}{
		{
			name:        "domain name",
			host:        "example.com",
			expireAfter: 2 * time.Hour,
		},
		{
			name:        "hostname",
			host:        "my-service",
			expireAfter: 2 * time.Hour,
		},
		{
			name:        "IP address",
			host:        "10.0.0.1",
			expireAfter: 2 * time.Hour,
			wantIP:      true,
		},
		{
			name:        "kubernetes service",
			host:        "my-service.default.svc",
			expireAfter: 2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certBytes, keyBytes, err := GenerateSelfSignedCertKeyWithOpts(tt.host, tt.expireAfter)
			assert.NoError(t, err)
			assert.NotEmpty(t, certBytes)
			assert.NotEmpty(t, keyBytes)

			block, _ := pem.Decode(certBytes)
			if assert.NotNil(t, block) {
				assert.Equal(t, CertificateBlockType, block.Type)
			}

			keyBlock, _ := pem.Decode(keyBytes)
			if assert.NotNil(t, keyBlock) {
				assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)
			}

			leaf := parseLeafCertificate(t, certBytes)
			assert.True(t, leaf.NotAfter.After(time.Now()))

			if tt.wantIP {
				assert.Len(t, leaf.IPAddresses, 1)
				assert.True(t, leaf.IPAddresses[0].Equal(net.ParseIP(tt.host)))
				assert.Empty(t, leaf.DNSNames)
			} else {
				assert.Contains(t, leaf.DNSNames, tt.host)
			}

			_, err = tls.X509KeyPair(certBytes, keyBytes)
			assert.NoError(t, err)
		})
	}
}

func TestCurrentCertKeyContentClones(t *testing.T) {
	fake := newSecretControllerFake()

	provider, err := NewSNIProviderForCname("test-provider", []string{"example.com"}, fake)
	require.NoError(t, err)

	provider.contentMu.Lock()
	provider.cert = []byte("cert-data")
	provider.key = []byte("key-data")
	provider.contentMu.Unlock()

	cert, key := provider.CurrentCertKeyContent()
	assert.Equal(t, []byte("cert-data"), cert)
	assert.Equal(t, []byte("key-data"), key)

	// mutate the cert data so we can later test that the source data is not mutated
	cert[0] = 'x'
	key[0] = 'y'

	cert2, key2 := provider.CurrentCertKeyContent()
	assert.Equal(t, []byte("cert-data"), cert2)
	assert.Equal(t, []byte("key-data"), key2)
}

func TestAddListenerAndNotify(t *testing.T) {
	fake := newSecretControllerFake()

	provider, err := NewSNIProviderForCname("test-provider", []string{"example.com"}, fake)
	require.NoError(t, err)

	var count int
	provider.AddListener(testListener{enqueueFunc: func() { count++ }})
	provider.AddListener(testListener{enqueueFunc: func() { count++ }})

	provider.notify()
	assert.Equal(t, 2, count)
}

func TestOnChange(t *testing.T) {
	tests := []struct {
		name          string
		secret        *corev1.Secret
		expectCreate  bool
		expectNotify  bool
		wantErr       string
		wantCert      []byte
		wantKey       []byte
		wantNilResult bool
	}{
		{
			name:          "nil secret recreates content",
			secret:        nil,
			expectCreate:  true,
			expectNotify:  true,
			wantNilResult: true,
		},
		{
			name: "valid secret updates content",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("cert-data"),
					corev1.TLSPrivateKeyKey: []byte("key-data"),
				},
			},
			expectNotify: true,
			wantCert:     []byte("cert-data"),
			wantKey:      []byte("key-data"),
		},
		{
			name: "missing cert errors",
			secret: &corev1.Secret{
				Data: map[string][]byte{corev1.TLSPrivateKeyKey: []byte("key")},
			},
			wantErr: corev1.TLSCertKey,
		},
		{
			name: "missing key errors",
			secret: &corev1.Secret{
				Data: map[string][]byte{corev1.TLSCertKey: []byte("cert")},
			},
			wantErr: corev1.TLSPrivateKeyKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newSecretControllerFake()

			provider, err := NewSNIProviderForCname("test-provider", []string{"example.com"}, fake)
			require.NoError(t, err)

			var notified bool
			provider.AddListener(testListener{enqueueFunc: func() { notified = true }})

			result, err := provider.onChange("", tt.secret)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
				assert.Equal(t, tt.expectNotify, notified)
				return
			}

			assert.NoError(t, err)
			if tt.wantNilResult {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.secret, result)
			}
			assert.Equal(t, tt.expectCreate, fake.createCount)
			assert.Equal(t, tt.expectNotify, notified)

			if tt.wantCert != nil || tt.wantKey != nil {
				cert, key := provider.CurrentCertKeyContent()
				assert.Equal(t, tt.wantCert, cert)
				assert.Equal(t, tt.wantKey, key)
			}

			if tt.expectCreate {
				secret, ok := fake.stored(Namespace, provider.secretName)
				if assert.True(t, ok) {
					assert.Equal(t, provider.secretName, secret.Name)
					assert.Equal(t, Namespace, secret.Namespace)
					assert.Equal(t, provider.name, secret.Labels[SecretLabelProvider])
				}
			}
		})
	}
}

func TestWillCertExpireWithinDuration(t *testing.T) {
	mustGenerateSecret := func(t *testing.T, lifetime time.Duration) *corev1.Secret {
		t.Helper()

		cert, key, err := GenerateSelfSignedCertKeyWithOpts("example.com", lifetime)
		assert.NoError(t, err)

		return &corev1.Secret{
			Data: map[string][]byte{
				corev1.TLSCertKey:       cert,
				corev1.TLSPrivateKeyKey: key,
			},
		}
	}

	tests := []struct {
		name              string
		secret            *corev1.Secret
		duration          time.Duration
		expectsExpiration bool
		expectedErr       string
	}{
		{
			name:              "expiring within duration",
			secret:            mustGenerateSecret(t, 30*time.Minute),
			duration:          time.Hour,
			expectsExpiration: true,
		},
		{
			name:              "not expiring within duration",
			secret:            mustGenerateSecret(t, 3*time.Hour),
			duration:          time.Hour,
			expectsExpiration: false,
		},
		{
			name:        "missing cert errors",
			secret:      &corev1.Secret{Data: map[string][]byte{}},
			duration:    time.Hour,
			expectedErr: corev1.TLSCertKey,
		},
		{
			name:        "missing key errors",
			secret:      &corev1.Secret{Data: map[string][]byte{corev1.TLSCertKey: []byte("cert")}},
			duration:    time.Hour,
			expectedErr: corev1.TLSPrivateKeyKey,
		},
	}

	fake := newSecretControllerFake()

	provider, err := NewSNIProviderForCname("test-provider", []string{"example.com"}, fake)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			willExpire, err := provider.willCertExpireWithinDuration(tt.secret, tt.duration)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.False(t, willExpire)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectsExpiration, willExpire)
		})
	}
}

func TestHandleCertCreatesSecret(t *testing.T) {
	fake := newSecretControllerFake()

	provider, err := NewSNIProviderForCname("test-provider", []string{"example.com"}, fake)
	require.NoError(t, err)

	err = provider.handleCert()
	assert.NoError(t, err)
	assert.True(t, fake.createCount)

	secret, ok := fake.stored(Namespace, provider.secretName)
	if assert.True(t, ok) {
		assert.NotEmpty(t, secret.Data[corev1.TLSCertKey])
		assert.NotEmpty(t, secret.Data[corev1.TLSPrivateKeyKey])
	}
}

func TestHandleCert(t *testing.T) {
	tests := []struct {
		name         string
		expireAfter  time.Duration
		expectUpdate bool
	}{
		{
			name:         "expiring secret is updated",
			expireAfter:  time.Minute * 30,
			expectUpdate: true,
		},
		{
			name:         "non-expiring secret is not updated",
			expireAfter:  time.Hour * 3,
			expectUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newSecretControllerFake()

			provider, err := NewSNIProviderForCname("test-provider", []string{"example.com"}, fake)
			require.NoError(t, err)

			initialCert, initialKey, err := GenerateSelfSignedCertKeyWithOpts("example.com", tt.expireAfter)
			require.NoError(t, err)

			seed := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: provider.secretName, Namespace: Namespace},
				Data: map[string][]byte{
					corev1.TLSCertKey:       initialCert,
					corev1.TLSPrivateKeyKey: initialKey,
				},
			}

			fake.mu.Lock()
			fake.secrets[fake.key(Namespace, provider.secretName)] = seed.DeepCopy()
			fake.mu.Unlock()

			err = provider.handleCert()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectUpdate, fake.updateCount)

			secret, ok := fake.stored(Namespace, provider.secretName)
			assert.True(t, ok)

			if tt.expectUpdate {
				assert.NotEqual(t, initialCert, secret.Data[corev1.TLSCertKey])
				assert.NotEqual(t, initialKey, secret.Data[corev1.TLSPrivateKeyKey])
			} else {
				assert.Equal(t, initialCert, secret.Data[corev1.TLSCertKey])
				assert.Equal(t, initialKey, secret.Data[corev1.TLSPrivateKeyKey])
			}
		})
	}
}

type testListener struct {
	enqueueFunc func()
}

func (t testListener) Enqueue() {
	if t.enqueueFunc != nil {
		t.enqueueFunc()
	}
}
