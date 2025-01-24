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

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
)

func TestMigrationChanges(t *testing.T) {
	m := exampleMigration{}

	result, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{})
	require.NoError(t, err)

	want := &migrations.MigrationChanges{
		Changes: []changes.ResourceChange{
			{
				Operation: changes.OperationPatch,
				Patch: &changes.PatchChange{
					ResourceRef: changes.ResourceReference{
						ObjectRef: types.NamespacedName{
							Name:      "test-svc",
							Namespace: "default",
						},
						Resource: "services",
						Version:  "v1",
					},
					Operations: []changes.PatchOperation{
						{
							Operation: "replace",
							Path:      "/spec/ports/0/targetPort",
							Value:     9371,
						},
					},
					Type: changes.PatchApplicationJSON,
				},
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
