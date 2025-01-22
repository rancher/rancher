package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const fieldMgr string = "rancher-migrations"

type configMapClient interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error)
	Apply(ctx context.Context, configMap *applycorev1.ConfigMapApplyConfiguration, opts metav1.ApplyOptions) (*corev1.ConfigMap, error)
}

// ConfigMapClient is an implementation of the MigrationStatus client that uses
// a ConfigMap to record the status of a migration.
type ConfigMapStatusClient struct {
	client configMapClient
}

// NewStatusClient creates and returns a new ConfigMapStatusClient ready for
// use.
func NewStatusClient(c typedcorev1.CoreV1Interface) *ConfigMapStatusClient {
	return &ConfigMapStatusClient{client: c.ConfigMaps(migrationsNamespace)}
}

// StatusFor looks up the migration status in a ConfigMap.
//
// If the ConfigMap does not contain the status for the migration, nil is
// returned.
//
// If the ConfigMap does not exist, this is not considered to be an error.
func (c *ConfigMapStatusClient) StatusFor(ctx context.Context, name string) (*MigrationStatus, error) {
	cm, err := c.client.Get(ctx, migrationsCMName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting migration status information: %w", err)
	}

	data := cm.Data[name]
	if data == "" {
		return nil, nil
	}

	var status MigrationStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("parsing migration status for %q: %w", name, err)
	}

	return &status, nil
}

// SetStatusFor records the status for a migration.
func (c *ConfigMapStatusClient) SetStatusFor(ctx context.Context, name string, status MigrationStatus) error {
	b, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("converting MigrationStatus to JSON: %w", err)
	}

	configMap, err := c.client.Get(ctx, migrationsCMName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("updating migration status information: %w", err)
		}
		configMap = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: migrationsCMName, Namespace: migrationsNamespace}}
	}

	configMapApplyConfig, err := applycorev1.ExtractConfigMap(configMap, fieldMgr)
	if err != nil {
		return fmt.Errorf("updating migration status config: %w", err)
	}

	configMapApplyConfig.
		WithData(map[string]string{
			name: string(b),
		})

	_, err = c.client.Apply(ctx, configMapApplyConfig, metav1.ApplyOptions{FieldManager: fieldMgr})

	return fmt.Errorf("updating migration status config: %w", err)
}
