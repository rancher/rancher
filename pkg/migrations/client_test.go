package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStatusFor(t *testing.T) {
	t.Run("when no config map exists", func(t *testing.T) {
		clientset := fake.NewClientset()
		client := NewStatusClient(clientset.CoreV1())

		result, err := client.StatusFor(context.TODO(), "test-migration")
		require.NoError(t, err)

		assert.Nil(t, result)
	})

	t.Run("when config map exists but no entry for the migration", func(t *testing.T) {
		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{}))
		client := NewStatusClient(clientset.CoreV1())

		result, err := client.StatusFor(context.TODO(), "test-migration")
		require.NoError(t, err)

		assert.Nil(t, result)
	})

	t.Run("when config map exists with invalid entry for the migration", func(t *testing.T) {
		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{
			"test-migration": "{",
		}))
		client := NewStatusClient(clientset.CoreV1())

		result, err := client.StatusFor(context.TODO(), "test-migration")
		assert.ErrorContains(t, err, `migration status for "test-migration"`)

		assert.Nil(t, result)
	})

	t.Run("when config map contains migration status", func(t *testing.T) {
		now := time.Now()
		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{
			"test-migration": testMarshalJSON(t, MigrationStatus{
				AppliedAt: now,
				Metrics: &changes.ApplyMetrics{
					Patch: 2,
				},
			}),
		}))
		client := NewStatusClient(clientset.CoreV1())

		result, err := client.StatusFor(context.TODO(), "test-migration")
		require.NoError(t, err)

		want := &MigrationStatus{
			AppliedAt: now,
			Metrics: &changes.ApplyMetrics{
				Patch: 2,
			},
		}
		assert.EqualExportedValues(t, want, result)
	})
}

func TestSetStatusFor(t *testing.T) {
	t.Run("when no config map exists", func(t *testing.T) {
		clientset := fake.NewClientset()
		client := NewStatusClient(clientset.CoreV1())
		appliedStatus := MigrationStatus{
			AppliedAt: time.Now(),
			Metrics: &changes.ApplyMetrics{
				Patch: 2,
			},
		}
		err := client.SetStatusFor(context.TODO(), "test-migration", appliedStatus)
		require.NoError(t, err)

		status, err := client.StatusFor(context.TODO(), "test-migration")
		assert.EqualExportedValues(t, &appliedStatus, status)
	})

	t.Run("setting multiple statuses", func(t *testing.T) {
		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{}))
		client := NewStatusClient(clientset.CoreV1())
		now := time.Now()

		firstMigration := MigrationStatus{
			AppliedAt: now, Metrics: &changes.ApplyMetrics{Create: 1}}
		secondMigration := MigrationStatus{
			AppliedAt: now.Add(time.Minute * -30), Metrics: &changes.ApplyMetrics{Delete: 1}}

		err := client.SetStatusFor(context.TODO(), "test-migration", firstMigration)
		assert.NoError(t, err)

		err = client.SetStatusFor(context.TODO(), "other-migration", secondMigration)
		assert.NoError(t, err)

		status, err := client.StatusFor(context.TODO(), "test-migration")
		assert.EqualExportedValues(t, &firstMigration, status)

		status, err = client.StatusFor(context.TODO(), "other-migration")
		assert.EqualExportedValues(t, &secondMigration, status)
	})
}
