package batched

import (
	"context"
	"testing"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestMigrationBatches(t *testing.T) {
	m := batchedMigration{}

	result1, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{})
	require.NoError(t, err)

	want := &migrations.MigrationChanges{
		Continue: "{\"start\":1}",
		Changes: []migrations.ChangeSet{
			{
				{
					Operation: changes.OperationCreate,
					Create: &changes.CreateChange{
						Resource: test.ToUnstructured(t, test.NewService(func(s *corev1.Service) {
							s.Name = "test-0"
						})),
					},
				},
			},
		},
	}
	assert.Equal(t, want, result1)

	result2, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{Continue: result1.Continue})
	require.NoError(t, err)

	want = &migrations.MigrationChanges{
		// No Continue
		Changes: []migrations.ChangeSet{
			{
				{
					Operation: changes.OperationCreate,
					Create: &changes.CreateChange{
						Resource: test.ToUnstructured(t, test.NewService(func(s *corev1.Service) {
							s.Name = "test-1"
						})),
					},
				},
			},
		},
	}
	assert.Equal(t, want, result2)
}

func TestMigrationBatchesCompletes(t *testing.T) {
	m := batchedMigration{}
	var batchCount int

	var continueString string

	for {
		result, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{Continue: continueString})
		require.NoError(t, err)
		batchCount += 1

		if result.Continue == "" {
			break
		}
		continueString = result.Continue
	}

	assert.Equal(t, 2, batchCount)
}

func newFakeClient(t *testing.T, objs ...runtime.Object) *fake.FakeDynamicClient {
	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))
	return fake.NewSimpleDynamicClient(testScheme, objs...)
}
