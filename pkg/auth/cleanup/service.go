// Package cleanup defines a type that represents a cleanup routine for an auth provider.
package cleanup

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
)

// Service performs cleanup of resources associated with a particular auth provider.
type Service struct {
	secretsInterface corev1.SecretInterface
}

// NewCleanupService creates and returns a new auth provider cleanup service.
func NewCleanupService(secretsInterface corev1.SecretInterface) *Service {
	return &Service{secretsInterface: secretsInterface}
}

// Run takes an auth config and checks if its auth provider is disabled, and ensures that any resources associated with it,
// like secrets, are deleted.
func (c *Service) Run(config *v3.AuthConfig) error {
	err := secrets.CleanupClientSecrets(c.secretsInterface, config)
	if err != nil {
		return fmt.Errorf("error cleaning up resources associated with a disabled auth provider %s: %w", config.Name, err)
	}
	return nil
}
