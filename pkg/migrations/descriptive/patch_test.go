package descriptive

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/rancher/rancher/pkg/migrations/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestApplyPatchChanges(t *testing.T) {
	ns := test.ToUnstructured(t, &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
			Labels: map[string]string{
				"example.com/testing": "testing",
			},
		},
	})
	change := PatchChange{
		ResourceRef: ResourceReference{
			ObjectRef: types.NamespacedName{
				Name: "test-ns",
			},
			Resource: "namespaces",
			Version:  "v1",
		},
		Operations: []PatchOperation{
			{
				Operation: "add",
				Path:      "/metadata/labels/example.com~1migration",
				Value:     "migrated",
			},
		},
		Type: PatchApplicationJSON,
	}

	result, err := ApplyPatchChanges(ns, change)
	require.NoError(t, err)

	want := test.ToUnstructured(t, &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
			Labels: map[string]string{
				"example.com/testing":   "testing",
				"example.com/migration": "migrated",
			},
		},
	})
	if diff := cmp.Diff(want, result); diff != "" {
		t.Errorf("failed to apply change: diff -want +got\n%s", diff)
	}
}

func TestApplyPatchChangesUnknownPatchType(t *testing.T) {
	ns := test.ToUnstructured(t, &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
			Labels: map[string]string{
				"example.com/testing": "testing",
			},
		},
	})
	change := PatchChange{
		ResourceRef: ResourceReference{
			ObjectRef: types.NamespacedName{
				Name: "test-ns",
			},
			Resource: "namespaces",
			Version:  "v1",
		},
		Operations: []PatchOperation{
			{
				Operation: "add",
				Path:      "/metadata/labels/example.com~1migration",
				Value:     "migrated",
			},
		},
		Type: "unknown",
	}

	_, err := ApplyPatchChanges(ns, change)
	require.ErrorContains(t, err, `unknown patch type: "unknown"`)
}
