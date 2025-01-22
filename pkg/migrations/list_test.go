package migrations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestList(t *testing.T) {
	t.Run("no migrations registered", func(t *testing.T) {
		clientset := fake.NewClientset()

		migrations, err := List(context.TODO(), NewStatusClient(clientset.CoreV1()))
		require.NoError(t, err)

		require.Empty(t, migrations)
	})

	t.Run("migration not applied", func(t *testing.T) {
		Register(testMigration{})
		defer func() {
			knownMigrations = nil
		}()
		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{}))

		migrations, err := List(context.TODO(), NewStatusClient(clientset.CoreV1()))
		require.NoError(t, err)

		want := []*MigrationInfo{
			{Name: "test-migration"},
		}
		require.Equal(t, want, migrations)
	})

	t.Run("migration with status", func(t *testing.T) {
		Register(testMigration{})
		defer func() {
			knownMigrations = nil
		}()

		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{
			"test-migration": testMarshalJSON(t, MigrationStatus{
				AppliedAt: time.Now(),
			}),
		}))

		migrations, err := List(context.TODO(), NewStatusClient(clientset.CoreV1()))
		require.NoError(t, err)

		want := []*MigrationInfo{
			{Name: "test-migration", Applied: true},
		}
		require.Equal(t, want, migrations)
	})

	t.Run("when the status ConfigMap does not exist", func(t *testing.T) {
		Register(testMigration{})
		defer func() {
			knownMigrations = nil
		}()
		clientset := fake.NewClientset()

		migrations, err := List(context.TODO(), NewStatusClient(clientset.CoreV1()))
		require.NoError(t, err)

		want := []*MigrationInfo{
			{Name: "test-migration"},
		}
		require.Equal(t, want, migrations)

	})

	t.Run("when the status ConfigMap is corrupt", func(t *testing.T) {
		Register(testMigration{})
		defer func() {
			knownMigrations = nil
		}()

		clientset := fake.NewClientset(newConfigMap("rancher-migrations", map[string]string{
			"test-migration": "testing",
		}))

		_, err := List(context.TODO(), NewStatusClient(clientset.CoreV1()))
		require.ErrorContains(t, err, `parsing migration status for "test-migration"`)
	})
}

func newConfigMap(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: migrationsNamespace,
		},
		Data: data,
	}
}

func testMarshalJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	_ = assert.NoError(t, err)

	return string(b)
}
