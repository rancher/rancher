package sample

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
)

func TestMigrationChange(t *testing.T) {
	ns := test.ToUnstructured(t, newNamespace("test-ns",
		withLabels(map[string]string{"kubernetes.io/metadata.name": "p-5djc7"})))
	change := changes.PatchChange{
		ResourceRef: changes.ResourceReference{
			ObjectRef: types.NamespacedName{
				Name: "test-ns",
			},
			Resource: "namespaces",
			Version:  "v1",
		},
		Operations: []changes.PatchOperation{
			{
				Operation: "add",
				Path:      "/metadata/labels/example.com~1migration",
				Value:     "migrated",
			},
		},
		Type: changes.PatchApplicationJSON,
	}

	result, err := changes.ApplyPatchChanges(ns, change)
	require.NoError(t, err)

	want := test.ToUnstructured(t, newNamespace("test-ns",
		withLabels(map[string]string{
			"example.com/migration":       "migrated",
			"kubernetes.io/metadata.name": "p-5djc7",
		})))
	if diff := cmp.Diff(want, result); diff != "" {
		t.Errorf("failed to apply change: diff -want +got\n%s", diff)
	}

}

func TestMigrationChanges(t *testing.T) {
	m := namespaceMigration{}
	client := newFakeClient(t,
		newNamespace("p-5djc7", withAnnotations(map[string]string{"lifecycle.cattle.io/create.namespace-auth": "true"})),
		newNamespace("p-q5gbl"),
	)

	result, err := m.Changes(context.TODO(), changes.ClientFrom(client), migrations.MigrationOptions{})
	require.NoError(t, err)

	want := &migrations.MigrationChanges{
		Changes: []migrations.ChangeSet{
			{
				{
					Operation: changes.OperationPatch,
					Patch: &changes.PatchChange{
						ResourceRef: changes.ResourceReference{
							ObjectRef: types.NamespacedName{
								Name: "p-5djc7",
							},
							Resource: "namespaces",
							Version:  "v1",
						},
						Operations: []changes.PatchOperation{
							{
								Operation: "add",
								Path:      "/metadata/labels/example.com~1migration",
								Value:     "migrated",
							},
						},
						Type: changes.PatchApplicationJSON,
					},
				},
			},
		},
	}
	assert.Equal(t, want, result)
}

func newFakeClient(t *testing.T, objs ...runtime.Object) *fake.FakeDynamicClient {
	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))
	return fake.NewSimpleDynamicClient(testScheme, objs...)
}

func withLabels(labels map[string]string) func(*corev1.Namespace) {
	return func(ns *corev1.Namespace) {
		ns.SetLabels(labels)
	}
}

func withAnnotations(annotations map[string]string) func(*corev1.Namespace) {
	return func(ns *corev1.Namespace) {
		ns.SetAnnotations(annotations)
	}
}

func newNamespace(name string, opts ...func(*corev1.Namespace)) *corev1.Namespace {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, opt := range opts {
		opt(ns)
	}

	return ns
}
