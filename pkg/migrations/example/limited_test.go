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

	require.Len(t, result1.Changes, 5)

	// Limit 2
	result2, err := m.Changes(context.TODO(), changes.ClientFrom(newFakeClient(t)), migrations.MigrationOptions{Limit: 2})
	require.NoError(t, err)

	require.Len(t, result2.Changes, 2)
}
