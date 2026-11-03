package managedchart

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"sync"
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/fleet/pkg/helmvalues"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// fakeChartGetter returns a minimal valid empty gzip+tar archive for any chart
// request, which causes the handler to produce a Bundle with no resources.
type fakeChartGetter struct{}

func (fakeChartGetter) Chart(_, _, _, _ string, _ bool) (io.ReadCloser, error) {
	return emptyTarGz()
}

// emptyTarGz creates a gzip-compressed tar archive with no entries.
func emptyTarGz() (io.ReadCloser, error) {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// fakeBundleCache always returns NotFound so updateStatus becomes a no-op in
// tests.
type fakeBundleCache struct{}

var _ fleetcontrollers.BundleCache = fakeBundleCache{}

func (fakeBundleCache) Get(namespace, name string) (*v1alpha1.Bundle, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "bundles"}, name)
}

func (fakeBundleCache) List(_ string, _ labels.Selector) ([]*v1alpha1.Bundle, error) {
	return nil, nil
}

func (fakeBundleCache) AddIndexer(_ string, _ generic.Indexer[*v1alpha1.Bundle]) {}

func (fakeBundleCache) GetByIndex(_, _ string) ([]*v1alpha1.Bundle, error) { return nil, nil }

// fakeSecretClient is a minimal in-memory Secret client for testing.
type fakeSecretClient struct {
	mu      sync.Mutex
	secrets map[string]*corev1.Secret // key: "namespace/name"
}

func newFakeSecretClient() *fakeSecretClient {
	return &fakeSecretClient{
		secrets: make(map[string]*corev1.Secret),
	}
}

func (f *fakeSecretClient) key(namespace, name string) string {
	return namespace + "/" + name
}

func (f *fakeSecretClient) Create(secret *corev1.Secret) (*corev1.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := f.key(secret.Namespace, secret.Name)
	if _, exists := f.secrets[k]; exists {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, secret.Name)
	}
	f.secrets[k] = secret.DeepCopy()
	return secret, nil
}

func (f *fakeSecretClient) Update(secret *corev1.Secret) (*corev1.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := f.key(secret.Namespace, secret.Name)
	if _, exists := f.secrets[k]; !exists {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, secret.Name)
	}
	f.secrets[k] = secret.DeepCopy()
	return secret, nil
}

func (f *fakeSecretClient) Get(namespace, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := f.key(namespace, name)
	secret, exists := f.secrets[k]
	if !exists {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	return secret.DeepCopy(), nil
}

func (f *fakeSecretClient) List(_ string, _ metav1.ListOptions) (*corev1.SecretList, error) {
	return &corev1.SecretList{}, nil
}

func (f *fakeSecretClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	return nil
}

func (f *fakeSecretClient) DeleteCollection(_ string, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	return nil
}

func (f *fakeSecretClient) UpdateStatus(*corev1.Secret) (*corev1.Secret, error) {
	return nil, nil
}

func (f *fakeSecretClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (f *fakeSecretClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (*corev1.Secret, error) {
	return nil, nil
}

func (f *fakeSecretClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*corev1.Secret, *corev1.SecretList], error) {
	return f, nil
}

func newTestHandler() *handler {
	return &handler{
		charts:      fakeChartGetter{},
		bundleCache: fakeBundleCache{},
		secrets:     newFakeSecretClient(),
	}
}

func newGenericMap(t *testing.T, raw string) *v1alpha1.GenericMap {
	t.Helper()
	gm := &v1alpha1.GenericMap{}
	require.NoError(t, gm.UnmarshalJSON([]byte(raw)))
	return gm
}

func newManagedChart(name, ns string, values *v1alpha1.GenericMap) *v3.ManagedChart {
	return &v3.ManagedChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v3.ManagedChartSpec{
			Chart:    "my-chart",
			RepoName: "my-repo",
			Values:   values,
		},
	}
}

// TestOnChange_ValuesNotInPlainText verifies that when a ManagedChart has Helm
// values, they are NOT stored in plain text on Bundle.Spec.Helm.Values.
func TestOnChange_ValuesNotInPlainText(t *testing.T) {
	t.Parallel()

	mcc := newManagedChart("test-chart", "fleet-local", newGenericMap(t, `{"key":"secret-value"}`))
	h := newTestHandler()

	objs, _, err := h.OnChange(mcc, v3.ManagedChartStatus{})
	require.NoError(t, err)

	bundle := findBundle(t, objs)
	assert.Nil(t, bundle.Spec.Helm.Values,
		"Helm values must not be stored in plain text on the Bundle spec")
}

// TestOnChange_ValuesHashSet verifies that the Bundle's ValuesHash is set to
// the sha256 hash of the secret data when values are present.
func TestOnChange_ValuesHashSet(t *testing.T) {
	t.Parallel()

	values := newGenericMap(t, `{"foo":"bar"}`)
	mcc := newManagedChart("test-chart", "fleet-local", values)
	h := newTestHandler()

	objs, _, err := h.OnChange(mcc, v3.ManagedChartStatus{})
	require.NoError(t, err)

	bundle := findBundle(t, objs)
	require.NotEmpty(t, bundle.Spec.ValuesHash, "ValuesHash must be set when values are present")

	// Independently recompute the hash from the secret data and verify it
	// matches what the Bundle carries.
	secret := getSecretFromClient(t, h, bundle)
	expectedHash, err := helmvalues.HashValuesSecret(secret.Data)
	require.NoError(t, err)
	assert.Equal(t, expectedHash, bundle.Spec.ValuesHash)
}

// TestOnChange_SecretCreated verifies that a Secret with the Fleet bundle-values
// type is created with the Bundle and contains the values under "values.yaml".
func TestOnChange_SecretCreated(t *testing.T) {
	t.Parallel()

	mcc := newManagedChart("test-chart", "fleet-local", newGenericMap(t, `{"key":"value"}`))
	h := newTestHandler()

	objs, _, err := h.OnChange(mcc, v3.ManagedChartStatus{})
	require.NoError(t, err)

	bundle := findBundle(t, objs)
	secret := getSecretFromClient(t, h, bundle)

	assert.Equal(t, bundle.Name, secret.Name, "Secret must share the Bundle's name")
	assert.Equal(t, bundle.Namespace, secret.Namespace, "Secret must be in the same namespace")
	assert.Equal(t, v1alpha1.SecretTypeBundleValues, string(secret.Type),
		"Secret must use Fleet's bundle-values type")
	assert.Contains(t, secret.Data, "values.yaml",
		"Secret data must contain the 'values.yaml' key")
}

// TestOnChange_NoValues verifies that when a ManagedChart has no Helm values,
// no Secret is created and ValuesHash is empty.
func TestOnChange_NoValues(t *testing.T) {
	t.Parallel()

	mcc := newManagedChart("no-values", "fleet-local", nil)
	h := newTestHandler()

	objs, _, err := h.OnChange(mcc, v3.ManagedChartStatus{})
	require.NoError(t, err)

	bundle := findBundle(t, objs)
	assert.Empty(t, bundle.Spec.ValuesHash, "ValuesHash must be empty when there are no values")

	// Verify no secret was created
	_, err = h.secrets.Get(bundle.Namespace, bundle.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "no Secret must be created when there are no values")
}

// TestOnChange_TargetValues verifies that per-target Helm values are also moved
// to the Secret and not stored in plain text on the target overrides.
func TestOnChange_TargetValues(t *testing.T) {
	t.Parallel()

	mcc := newManagedChart("target-values", "fleet-local", newGenericMap(t, `{"global":"true"}`))
	mcc.Spec.Targets = []v1alpha1.BundleTarget{
		{
			Name: "cluster-a",
			BundleDeploymentOptions: v1alpha1.BundleDeploymentOptions{
				Helm: &v1alpha1.HelmOptions{
					Values: newGenericMap(t, `{"replica":2}`),
				},
			},
		},
	}
	h := newTestHandler()

	objs, _, err := h.OnChange(mcc, v3.ManagedChartStatus{})
	require.NoError(t, err)

	bundle := findBundle(t, objs)
	for _, target := range bundle.Spec.Targets {
		if target.Helm != nil {
			assert.Nil(t, target.Helm.Values,
				"target %q must not carry plain-text values", target.Name)
		}
	}

	secret := getSecretFromClient(t, h, bundle)
	assert.Contains(t, secret.Data, "values.yaml", "main values must be in the secret")
	assert.Contains(t, secret.Data, "cluster-a", "target values must be in the secret")
}

// findBundle returns the single Bundle from objs or fails the test.
func findBundle(t *testing.T, objs []runtime.Object) *v1alpha1.Bundle {
	t.Helper()
	for _, obj := range objs {
		if b, ok := obj.(*v1alpha1.Bundle); ok {
			return b
		}
	}
	t.Fatal("no Bundle found in returned objects")
	return nil
}

// getSecretFromClient retrieves the Secret from the handler's client that
// corresponds to the given Bundle, or fails the test if not found.
func getSecretFromClient(t *testing.T, h *handler, bundle *v1alpha1.Bundle) *corev1.Secret {
	t.Helper()
	secret, err := h.secrets.Get(bundle.Namespace, bundle.Name, metav1.GetOptions{})
	require.NoError(t, err, "Secret must exist in the client")
	return secret
}
