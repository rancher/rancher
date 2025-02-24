package example

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
)

func TestMigrationLimited(t *testing.T) {
	m := limitedMigration{}

	// No Limit
	result1, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{})
	require.NoError(t, err)

	require.Equal(t, 5, countChanges(result1.Changes))

	// Limit 2
	result2, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{Limit: 2})
	require.NoError(t, err)

	require.Equal(t, 2, countChanges(result2.Changes))
}

func countChanges(sets []migrations.ChangeSet) int {
	var count int
	for _, changeset := range sets {
		count += len(changeset)
	}

	return count
}
