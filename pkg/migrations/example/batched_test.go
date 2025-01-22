package example

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/descriptive"
	"github.com/rancher/rancher/pkg/migrations/test"
)

func TestMigrationBatches(t *testing.T) {
	m := batchedMigration{}

	result1, err := m.Changes(context.TODO(), descriptive.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{})
	require.NoError(t, err)

	want := &migrations.MigrationChanges{
		Continue: "{\"start\":1}",
		Changes: []descriptive.ResourceChange{
			{
				Operation: descriptive.OperationCreate,
				Create: &descriptive.CreateChange{
					Resource: test.ToUnstructured(t, test.NewService(func(s *corev1.Service) {
						s.Name = "test-0"
					})),
				},
			},
		},
	}
	assert.Equal(t, want, result1)

	result2, err := m.Changes(context.TODO(), descriptive.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{Continue: result1.Continue})
	require.NoError(t, err)

	want = &migrations.MigrationChanges{
		// No Continue
		Changes: []descriptive.ResourceChange{
			{
				Operation: descriptive.OperationCreate,
				Create: &descriptive.CreateChange{
					Resource: test.ToUnstructured(t, test.NewService(func(s *corev1.Service) {
						s.Name = "test-1"
					})),
				},
			},
		},
	}
	assert.Equal(t, want, result2)

}
