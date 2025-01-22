package descriptive

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rancher/rancher/pkg/migrations/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

func TestApplyChanges(t *testing.T) {
	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))

	t.Run("ApplyChanges patching an existing resource", func(t *testing.T) {
		svc := test.NewService()

		change := ResourceChange{
			Operation: OperationPatch,
			Patch: &PatchChange{
				ResourceRef: ResourceReference{
					ObjectRef: types.NamespacedName{
						Name:      svc.Name,
						Namespace: svc.Namespace,
					},
					Resource: "services",
					Version:  "v1",
				},
				Operations: []PatchOperation{
					{
						Operation: "replace",
						Path:      "/spec/ports/0/targetPort",
						Value:     9371,
					},
				},
				Type: PatchApplicationJSON,
			},
		}

		changes := []ResourceChange{
			change,
		}

		k8sClient := newFakeClient(testScheme, svc)

		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.NoError(t, err)

		updated, err := k8sClient.Resource(change.Patch.ResourceRef.GVR()).Namespace(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
		assert.NoError(t, err)

		want := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]any{
					"creationTimestamp": nil,
					"name":              "test-svc",
					"namespace":         "default",
				},
				"spec": map[string]any{
					"ports": []any{
						map[string]any{
							"name":       "http-80",
							"port":       int64(80),
							"protocol":   "TCP",
							"targetPort": int64(9371),
						},
					},
				},
				"status": map[string]any{
					"loadBalancer": map[string]any{},
				},
			},
		}
		if diff := cmp.Diff(want, updated); diff != "" {
			t.Errorf("failed to apply migrations: diff -want +got\n%s", diff)
		}
		wantMetrics := &ApplyMetrics{Patch: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges creating a new resource and updating an existing resource", func(t *testing.T) {
		ac := newAuthConfig()

		patchChange := ResourceChange{
			Operation: OperationPatch,
			Patch: &PatchChange{
				ResourceRef: ResourceReference{
					ObjectRef: types.NamespacedName{
						Name: "shibboleth",
					},
					Group:    "management.cattle.io",
					Resource: "authconfigs",
					Version:  "v3",
				},
				Operations: []PatchOperation{
					{
						Operation: "replace",
						Path:      "/openLdapConfig/serviceAccountPassword",
						Value:     "cattle-secrets:test-secret",
					},
				},
				Type: PatchApplicationJSON,
			},
		}
		changes := []ResourceChange{
			{
				Operation: OperationCreate,
				Create: &CreateChange{
					Resource: toUnstructured(t,
						newSecret(types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"}, map[string][]byte{"testing": []byte("TESTSECRET")})),
				},
			},
			patchChange,
		}

		k8sClient := newFakeClient(testScheme, ac)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.NoError(t, err)

		want := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata": map[string]any{
					"name": "shibboleth",
				},
				"type":    "shibbolethConfig",
				"enabled": true,
				"openLdapConfig": map[string]any{
					"serviceAccountPassword": "cattle-secrets:test-secret",
				},
			},
		}
		updated, err := k8sClient.Resource(patchChange.Patch.ResourceRef.GVR()).Get(context.TODO(), "shibboleth", metav1.GetOptions{})
		assert.NoError(t, err)
		if diff := cmp.Diff(want, updated); diff != "" {
			t.Errorf("failed to apply update existing AuthConfig: diff -want +got\n%s", diff)
		}

		created, err := k8sClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}).Namespace("cattle-secrets").Get(context.TODO(), "test-secret", metav1.GetOptions{})
		assert.NoError(t, err)
		wantSecret := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test-secret",
					"namespace": "cattle-secrets",
				},
				"data": map[string]any{
					"token": "testing",
				},
			},
		}
		if diff := cmp.Diff(wantSecret, created, unstructuredIgnore); diff != "" {
			t.Errorf("failed to apply migrations: diff -want +got\n%s", diff)
		}

		wantMetrics := &ApplyMetrics{Patch: 1, Create: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges deleting a resource", func(t *testing.T) {
		secret := newSecret(
			types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"},
			map[string][]byte{"testing": []byte("TESTSECRET")})
		secretRef := ResourceReference{
			ObjectRef: types.NamespacedName{
				Name:      "test-secret",
				Namespace: "cattle-secrets",
			},
			Group:    "",
			Resource: "secrets",
			Version:  "v1",
		}

		changes := []ResourceChange{
			{
				Operation: OperationDelete,
				Delete: &DeleteChange{
					ResourceRef: secretRef,
				},
			},
		}

		k8sClient := newFakeClient(testScheme, secret)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.NoError(t, err)

		_, err = k8sClient.Resource(secretRef.GVR()).Namespace(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
		require.True(t, apierrors.IsNotFound(err))

		wantMetrics := &ApplyMetrics{Delete: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges deleting a resource with dry-run", func(t *testing.T) {
		t.Skip("See https://github.com/kubernetes/kubernetes/issues/129737")
		secret := newSecret(
			types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"},
			map[string][]byte{"testing": []byte("TESTSECRET")})
		secretRef := ResourceReference{
			ObjectRef: types.NamespacedName{
				Name:      "test-secret",
				Namespace: "cattle-secrets",
			},
			Group:    "",
			Resource: "secrets",
			Version:  "v1",
		}

		changes := []ResourceChange{
			{
				Operation: OperationDelete,
				Delete: &DeleteChange{
					ResourceRef: secretRef,
				},
			},
		}

		k8sClient := newFakeClient(testScheme, secret)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{DryRun: true}, testMapper())
		require.NoError(t, err)

		actions := k8sClient.Fake.Actions()
		want := []clientgotesting.Action{
			clientgotesting.DeleteActionImpl{
				Name: "test-secret",
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: "cattle-secrets",
					Verb:      "delete",
					Resource: schema.GroupVersionResource{
						Version: "v1", Resource: "secrets"},
				},
				DeleteOptions: metav1.DeleteOptions{
					DryRun: []string{metav1.DryRunAll},
				},
			},
		}
		if diff := cmp.Diff(want, actions); diff != "" {
			t.Fatalf("unexpected actions: diff -want +got\n%s", diff)
		}
		wantMetrics := &ApplyMetrics{}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges creating a resource with no GVK", func(t *testing.T) {
		changes := []ResourceChange{
			{
				Operation: OperationCreate,
				Create: &CreateChange{
					Resource: &unstructured.Unstructured{
						Object: map[string]any{
							"metadata": map[string]any{
								"name": "shibboleth",
							},
							"type":    "shibbolethConfig",
							"enabled": true,
							"openLdapConfig": map[string]any{
								"serviceAccountPassword": "thisisatestpassword",
							},
						},
					},
				},
			},
		}

		k8sClient := newFakeClient(testScheme)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.ErrorContains(t, err, "GVK missing from resource: shibboleth")
		wantMetrics := &ApplyMetrics{Create: 1, Errors: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges creating a resource with unknown GVK", func(t *testing.T) {
		changes := []ResourceChange{
			{
				Operation: OperationCreate,
				Create: &CreateChange{
					Resource: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "management.cattle.io/v3",
							"kind":       "AuthConfig",
							"metadata": map[string]any{
								"name": "shibboleth",
							},
							"type":    "shibbolethConfig",
							"enabled": true,
							"openLdapConfig": map[string]any{
								"serviceAccountPassword": "thisisatestpassword",
							},
						},
					},
				},
			},
		}

		k8sClient := newFakeClient(testScheme)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.ErrorContains(t, err, "unable to get resource mapping for management.cattle.io/v3, Kind=AuthConfig")
		wantMetrics := &ApplyMetrics{Create: 1, Errors: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges creating a resource that already exists", func(t *testing.T) {
		secret := newSecret(types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"}, map[string][]byte{"testing": []byte("TESTSECRET")})
		changes := []ResourceChange{
			{
				Operation: OperationCreate,
				Create: &CreateChange{
					Resource: toUnstructured(t, secret),
				},
			},
		}

		k8sClient := newFakeClient(testScheme, secret)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.ErrorContains(t, err, `failed to apply Create change - creating resource: secrets "test-secret" already exists`)
		wantMetrics := &ApplyMetrics{Create: 1, Errors: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("ApplyChanges patching a resource with an invalid patch", func(t *testing.T) {
		svc := test.NewService()
		changes := []ResourceChange{
			{
				Operation: OperationPatch,
				Patch: &PatchChange{
					ResourceRef: ResourceReference{
						ObjectRef: types.NamespacedName{
							Name:      svc.Name,
							Namespace: svc.Namespace,
						},
						Resource: "services",
						Version:  "v1",
					},
					Operations: []PatchOperation{
						{
							Operation: "modify",
							Path:      "/spec/ports/0/targetPort",
							Value:     9371,
						},
					},
					Type: PatchApplicationJSON,
				},
			},
		}

		k8sClient := newFakeClient(testScheme, svc)
		metrics, err := ApplyChanges(context.TODO(), k8sClient, changes, ApplyOptions{}, testMapper())
		require.ErrorContains(t, err, `failed to apply Patch change - applying patch: Unexpected kind: modify`)
		wantMetrics := &ApplyMetrics{Patch: 1, Errors: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

}

func TestResourceChangeValidation(t *testing.T) {
	validationTests := map[string]struct {
		change    ResourceChange
		wantError string
	}{
		"Create with no CreateChange": {
			change:    ResourceChange{Operation: OperationCreate},
			wantError: "Create operation has no creation configuration",
		},
		"Patch with no PatchChange": {
			change:    ResourceChange{Operation: OperationPatch},
			wantError: "Patch operation has no patch configuration",
		},
	}

	for name, tt := range validationTests {
		t.Run(name, func(t *testing.T) {
			err := tt.change.Validate()
			var errMsg string
			if err != nil {
				errMsg = err.Error()
			}

			if diff := cmp.Diff(tt.wantError, errMsg); diff != "" {
				t.Errorf("Validate() failed: diff -want +got\n%s", diff)
			}
		})
	}
}

func newFakeClient(scheme *runtime.Scheme, objs ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClient(scheme, objs...)
}

func newSecret(name types.NamespacedName, data map[string][]byte, opts ...func(s *corev1.Secret)) *corev1.Secret {
	// TODO: convert data map[string]string to map[string][]byte
	s := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func toUnstructured(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	assert.NoError(t, err)

	return &unstructured.Unstructured{Object: raw}
}

func newAuthConfig() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "AuthConfig",
			"metadata": map[string]any{
				"name": "shibboleth",
			},
			"type":    "shibbolethConfig",
			"enabled": true,
			"openLdapConfig": map[string]any{
				"serviceAccountPassword": "thisisatestpassword",
			},
		},
	}
}

// This would normally be dynamically created from the API Server.
func testMapper() *meta.DefaultRESTMapper {
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.AddSpecific(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secret"},
		meta.RESTScopeNamespace)

	return mapper
}

var unstructuredIgnore = cmpopts.IgnoreMapEntries(func(k string, _ any) bool {
	return k != "creationTimestamp"
})
