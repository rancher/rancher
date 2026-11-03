package saml

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPerAssertionStorage(t *testing.T) {
	t.Parallel()

	t.Run("new ID is not seen", func(t *testing.T) {
		t.Parallel()
		store := newConfigMapIDStore(newFakeConfigMapClient(), nil)

		seen, err := store.seen("id-1", time.Now().Add(time.Minute))
		require.NoError(t, err)
		assert.False(t, seen)
	})

	t.Run("same ID seen twice is a replay", func(t *testing.T) {
		t.Parallel()
		store := newConfigMapIDStore(newFakeConfigMapClient(), nil)
		expiry := time.Now().Add(time.Minute)

		seen, err := store.seen("id-replay", expiry)
		require.NoError(t, err)
		assert.False(t, seen)

		seen, err = store.seen("id-replay", expiry)
		require.NoError(t, err)
		assert.True(t, seen)
	})

	t.Run("different IDs are tracked independently", func(t *testing.T) {
		t.Parallel()
		store := newConfigMapIDStore(newFakeConfigMapClient(), nil)
		expiry := time.Now().Add(time.Minute)

		seen, err := store.seen("id-a", expiry)
		require.NoError(t, err)
		assert.False(t, seen)

		seen, err = store.seen("id-b", expiry)
		require.NoError(t, err)
		assert.False(t, seen)

		seen, err = store.seen("id-a", expiry)
		require.NoError(t, err)
		assert.True(t, seen)

		seen, err = store.seen("id-b", expiry)
		require.NoError(t, err)
		assert.True(t, seen)
	})

	t.Run("each assertion gets its own ConfigMap", func(t *testing.T) {
		t.Parallel()
		fake := newFakeConfigMapClient()
		store := newConfigMapIDStore(fake, nil)
		expiry := time.Now().Add(time.Minute)

		store.seen("id-x", expiry) //nolint:errcheck
		store.seen("id-y", expiry) //nolint:errcheck

		fake.mu.Lock()
		count := len(fake.configMaps)
		fake.mu.Unlock()
		assert.Equal(t, 2, count)
	})

	t.Run("expired entry left by a previous process run is replaced, not a replay", func(t *testing.T) {
		t.Parallel()
		fake := newFakeConfigMapClient()
		store := newConfigMapIDStore(fake, nil)

		// Record an assertion with an already-past expiry.
		seen, err := store.seen("id-stale", time.Now().Add(-time.Second))
		require.NoError(t, err)
		assert.False(t, seen)

		// Simulate a restart: create a new store backed by the same storage.
		store2 := newConfigMapIDStore(fake, nil)

		// The expiry stored in the ConfigMap has passed; should clean up and treat as fresh.
		seen, err = store2.seen("id-stale", time.Now().Add(time.Minute))
		require.NoError(t, err)
		assert.False(t, seen)
	})

	t.Run("non-expired assertion survives a process restart", func(t *testing.T) {
		t.Parallel()
		fake := newFakeConfigMapClient()
		expiry := time.Now().Add(time.Minute)

		seen, err := newConfigMapIDStore(fake, nil).seen("id-persistent", expiry)
		require.NoError(t, err)
		assert.False(t, seen)

		// Simulate restart: new store instance, same backing storage.
		seen, err = newConfigMapIDStore(fake, nil).seen("id-persistent", expiry)
		require.NoError(t, err)
		assert.True(t, seen)
	})
}

func TestCleanUpExpiredAssertionIDs(t *testing.T) {
	now := time.Now()

	expiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "saml-assertion-expired", Namespace: "cattle-system",
			Labels: configMapLabels,
		},
		Data: map[string]string{
			"expiry": strconv.FormatInt(now.Add(-time.Minute).Unix(), 10),
		},
	}
	liveConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "saml-assertion-live", Namespace: "cattle-system",
			Labels: configMapLabels,
		},
		Data: map[string]string{
			"expiry": strconv.FormatInt(now.Add(time.Minute).Unix(), 10),
		},
	}
	nonAssertionConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "saml-assertion-non-assertion", Namespace: "cattle-system",
		},
		Data: map[string]string{
			"some": "value",
		},
	}

	tests := map[string]struct {
		configMaps     []*corev1.ConfigMap
		wantConfigMaps map[string]*corev1.ConfigMap
	}{
		"remove expired sessions": {
			configMaps:     []*corev1.ConfigMap{expiredConfigMap},
			wantConfigMaps: map[string]*corev1.ConfigMap{},
		},
		"remove only expired session": {
			configMaps: []*corev1.ConfigMap{expiredConfigMap, liveConfigMap},
			wantConfigMaps: map[string]*corev1.ConfigMap{
				"cattle-system/saml-assertion-live": liveConfigMap,
			},
		},
		"ignores non-assertion config maps": {
			configMaps: []*corev1.ConfigMap{expiredConfigMap, liveConfigMap, nonAssertionConfigMap},
			wantConfigMaps: map[string]*corev1.ConfigMap{
				"cattle-system/saml-assertion-live":          liveConfigMap,
				"cattle-system/saml-assertion-non-assertion": nonAssertionConfigMap,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fakeClient := newFakeConfigMapClient(tt.configMaps...)
			storage := newConfigMapIDStore(fakeClient, &fakeConfigMapCache{configMaps: tt.configMaps})
			ctx, cancel := context.WithCancel(t.Context())
			c := make(chan time.Time)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				storage.cleanUpExpiredAssertionIDs(ctx, c)
			}()

			c <- time.Unix(0, 0)
			cancel()
			wg.Wait()

			assert.Equal(t, tt.wantConfigMaps, fakeClient.configMaps)
		})
	}
}

func newFakeConfigMapClient(configmaps ...*corev1.ConfigMap) *fakeConfigMapClient {
	c := &fakeConfigMapClient{
		configMaps: make(map[string]*corev1.ConfigMap),
	}

	for _, cm := range configmaps {
		c.configMaps[cm.Namespace+"/"+cm.Name] = cm.DeepCopy()
	}

	return c
}

type fakeConfigMapClient struct {
	mu         sync.Mutex
	configMaps map[string]*corev1.ConfigMap
	afterGet   func() // called after Get, outside mutex
}

func (f *fakeConfigMapClient) Create(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.configMaps[cm.Namespace+"/"+cm.Name] != nil {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, cm.Name)
	}
	cm = cm.DeepCopy()
	cm.ResourceVersion = "1"
	f.configMaps[cm.Namespace+"/"+cm.Name] = cm

	return cm, nil
}

func (f *fakeConfigMapClient) Get(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
	f.mu.Lock()
	cm, ok := f.configMaps[namespace+"/"+name]
	f.mu.Unlock()
	if f.afterGet != nil {
		f.afterGet()
	}
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, name)
	}

	return cm, nil
}

func (f *fakeConfigMapClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := namespace + "/" + name
	if _, ok := f.configMaps[key]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, name)
	}
	delete(f.configMaps, key)

	return nil
}

type fakeConfigMapCache struct {
	mu         sync.Mutex
	configMaps []*corev1.ConfigMap
}

func (f *fakeConfigMapCache) List(namespace string, selector labels.Selector) ([]*corev1.ConfigMap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var result []*corev1.ConfigMap
	for _, cm := range f.configMaps {
		if cm.Namespace == namespace && selector.Matches(labels.Set(cm.Labels)) {
			result = append(result, cm)
		}
	}

	return result, nil
}
