package projectscopedsecrets

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
	"github.com/stretchr/testify/require"
)

var (
	secret = corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			Annotations: map[string]string{
				"field.cattle.io/projectId":                            "c-84xb8:p-4kbkc",
				"lifecycle.cattle.io/create.secretsController_c-84xb8": "true",
			},
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
			Finalizers: []string{"clusterscoped.controller.cattle.io/secretsController_c-84xb8"},
		},
	}
	migratedSecret = corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			Annotations: map[string]string{
				"field.cattle.io/projectId":                  "c-84xb8:p-4kbkc",
				"management.cattle.io/project-scoped-secret": "p-4kbkc",
			},
		},
	}
)

func TestMigrationChange(t *testing.T) {
	secret := test.ToUnstructured(t, secret.DeepCopy())

	change := changes.PatchChange{
		ResourceRef: changes.ResourceReference{
			ObjectRef: types.NamespacedName{
				Name: secret.GetName(),
			},
			Resource: "secrets",
			Version:  "v1",
		},
		Operations: []changes.PatchOperation{
			{
				Operation: "add",
				Path:      "/metadata/annotations/management.cattle.io~1project-scoped-secret",
				Value:     "p-4kbkc",
			},
			{
				Operation: "remove",
				Path:      "/metadata/labels/cattle.io~1creator",
			},
			{
				Operation: "remove",
				Path:      "/metadata/annotations/lifecycle.cattle.io~1create.secretsController_c-84xb8",
			},
			{
				Operation: "remove",
				Path:      "/metadata/finalizers/0",
			},
		},
		Type: changes.PatchApplicationJSON,
	}
	result, err := changes.ApplyPatchChanges(secret, change)
	require.NoError(t, err)

	want := test.ToUnstructured(t, migratedSecret.DeepCopy())

	if diff := cmp.Diff(want, result); diff != "" {
		t.Errorf("failed to apply change: diff -want +got\n%s", diff)
	}
}
