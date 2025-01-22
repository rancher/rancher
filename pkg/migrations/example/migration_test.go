package example

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/rancher/rancher/pkg/migrations/descriptive"
)

func TestMigrationChanges(t *testing.T) {
	m := exampleMigration{}

	result, err := m.Changes(context.TODO(), descriptive.ClientFrom(newFakeClient(t)))
	require.NoError(t, err)

	want := []descriptive.ResourceChange{
		{
			Operation: descriptive.OperationPatch,
			Patch: &descriptive.PatchChange{
				ResourceRef: descriptive.ResourceReference{
					ObjectRef: types.NamespacedName{
						Name:      "test-svc",
						Namespace: "default",
					},
					Resource: "services",
					Version:  "v1",
				},
				Operations: []descriptive.PatchOperation{
					{
						Operation: "replace",
						Path:      "/spec/ports/0/targetPort",
						Value:     9371,
					},
				},
				Type: descriptive.PatchApplicationJSON,
			},
		},
	}
	assert.Equal(t, want, result)
}

func TestMigrationChangesAlreadyApplied(t *testing.T) {
	t.Skip()
}

func newFakeClient(t *testing.T, objs ...runtime.Object) *fake.FakeDynamicClient {
	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))
	return fake.NewSimpleDynamicClient(testScheme, objs...)
}
