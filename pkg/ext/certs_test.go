package ext

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type watcher struct {
	name      string
	eventChan chan watch.Event
}

func (w *watcher) Stop() {
	close(w.eventChan)
}

func (w *watcher) ResultChan() <-chan watch.Event {
	return w.eventChan
}

type mapStore[T runtime.Object] struct {
	mu sync.RWMutex
	m  map[string]T

	watchMu sync.RWMutex
	watches []*watcher
}

func (s *mapStore[T]) Get(n string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.m[n]; ok {
		return v, nil
	}

	var v T
	return v, fmt.Errorf("not found")
}

func (s *mapStore[T]) Create(n string, v T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[n]; ok {
		return apierrors.NewAlreadyExists(schema.GroupResource{}, n)
	}

	s.m[n] = v

	s.handleEvent(n, watch.Added)

	return nil
}

func (s *mapStore[T]) Update(n string, v T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[n]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{}, n)
	}

	s.m[n] = v
	s.handleEvent(n, watch.Added)

	return nil
}

func (s *mapStore[T]) Delete(n string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[n]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{}, n)
	}

	delete(s.m, n)

	return nil
}

func (s *mapStore[T]) Watch(name string) watch.Interface {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	eventChan := make(chan watch.Event, 100)
	watcher := &watcher{
		name:      name,
		eventChan: eventChan,
	}
	s.watches = append(s.watches, watcher)

	defer func() {
		if v, ok := s.m[name]; ok {
			eventChan <- watch.Event{
				Type:   watch.Added,
				Object: v,
			}
		}
	}()

	return watcher
}

func (s *mapStore[T]) handleEvent(name string, kind watch.EventType) {
	s.watchMu.RLock()
	defer s.watchMu.RUnlock()

	v := s.m[name]
	for _, w := range s.watches {
		if w.name == name {
			w.eventChan <- watch.Event{
				Type:   kind,
				Object: v,
			}
		}
	}
}

func setupMockController(t *testing.T, store *mapStore[*corev1.Secret]) *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList] {
	t.Helper()

	get := func(namespace string, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
		secret, err := store.Get(fmt.Sprintf("%s/%s", namespace, name))
		if err != nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		return secret, nil
	}
	create := func(secret *corev1.Secret) (*corev1.Secret, error) {
		if err := store.Create(fmt.Sprintf("%s/%s", secret.Namespace, secret.Name), secret); err != nil {
			return nil, apierrors.NewAlreadyExists(schema.GroupResource{}, secret.Name)
		}
		return secret, nil
	}
	update := func(secret *corev1.Secret) (*corev1.Secret, error) {
		if err := store.Update(fmt.Sprintf("%s/%s", secret.Namespace, secret.Name), secret); err != nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, secret.Name)
		}

		return secret, nil
	}
	delete := func(namespace string, name string, _ *metav1.DeleteOptions) error {
		if err := store.Delete(fmt.Sprintf("%s/%s", namespace, name)); err != nil {
			return apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		return nil
	}
	watch := func(namespace string, _ metav1.ListOptions) (watch.Interface, error) {
		watcher := store.Watch(fmt.Sprintf("%s/%s", namespace, ""))

		return watcher, nil
	}

	controller := gomock.NewController(t)
	secretMock := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](controller)
	secretMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(get).AnyTimes()
	secretMock.EXPECT().Create(gomock.Any()).DoAndReturn(create).AnyTimes()
	secretMock.EXPECT().Update(gomock.Any()).DoAndReturn(update).AnyTimes()
	secretMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(delete).AnyTimes()
	secretMock.EXPECT().Watch(gomock.Any(), gomock.Any()).DoAndReturn(watch).AnyTimes()

	return secretMock
}

func setup(t *testing.T) (*rotatingSNIProvider, *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList], *mapStore[*corev1.Secret]) {
	t.Helper()

	store := &mapStore[*corev1.Secret]{
		m: make(map[string]*corev1.Secret),
	}
	secretMock := setupMockController(t, store)

	provider, err := NewSNIProviderForCname(
		"test-provider",
		[]string{
			fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace),
		},
		secretMock)
	assert.NoError(t, err)

	return provider, secretMock, store
}

func TestRotatingSNIProviderNoExistingSecret(t *testing.T) {
	provider, secretMock, store := setup(t)
	stopChan := make(chan struct{})

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		err := provider.Run(stopChan)
		assert.NoError(t, err)

		wg.Done()
	}()

	wait.Until(func() {
		_, err := secretMock.Get(Namespace, provider.secretName, metav1.GetOptions{})

		if err == nil {
			close(stopChan)
		} else {
			assert.NoError(t, client.IgnoreNotFound(err))
		}
	}, time.Second*1, stopChan)

	wg.Wait()

	secret, err := store.Get(fmt.Sprintf("%s/%s", Namespace, provider.secretName))
	assert.Error(t, err)
	assert.Nil(t, secret)
}

func TestRotatingSNIProviderWithExistingExpiringSecret(t *testing.T) {
	provider, secretMock, store := setup(t)
	stopChan := make(chan struct{})

	initialCert, initialCa, err := GenerateSelfSignedCertKeyWithOpts(fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace), time.Second*5)
	assert.NoError(t, err)

	err = store.Create(fmt.Sprintf("%s/%s", Namespace, provider.secretName), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      provider.secretName,
			Namespace: Namespace,
		},
		Data: map[string][]byte{
			corev1.TLSCertKey:       initialCert,
			corev1.TLSPrivateKeyKey: initialCa,
		},
	})
	assert.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		err := provider.Run(stopChan)
		assert.NoError(t, err)

		wg.Done()
	}()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		secret, err := secretMock.Get(Namespace, provider.secretName, metav1.GetOptions{})
		assert.NoError(t, err)

		cert := secret.Data[corev1.TLSCertKey]
		ca := secret.Data[corev1.TLSPrivateKeyKey]
		if !bytes.Equal(initialCert, cert) && !bytes.Equal(initialCa, ca) {
			cancel()
		}
	}, time.Second)

	secret, err := secretMock.Get(Namespace, provider.secretName, metav1.GetOptions{})
	assert.NoError(t, err)

	cert := secret.Data[corev1.TLSCertKey]
	assert.NotEqual(t, initialCert, cert)

	ca := secret.Data[corev1.TLSPrivateKeyKey]
	assert.NotEqual(t, initialCa, ca)

	close(stopChan)

	wg.Wait()

	secret, err = store.Get(fmt.Sprintf("%s/%s", Namespace, provider.secretName))
	assert.Error(t, err)
	assert.Nil(t, secret)
}
