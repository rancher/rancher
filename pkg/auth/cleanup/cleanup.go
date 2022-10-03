// Package cleanup defines a type that represents a cleanup routine for an auth provider.
package cleanup

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
)

// Service performs cleanup of resources associated with a particular auth provider.
type Service struct{}

// NewCleanupService creates and returns a new auth provider cleanup service.
func NewCleanupService() *Service {
	return &Service{}
}

// TriggerCleanup takes an auth config and checks if its auth provider is disabled, and ensures that any resources associated with it,
// like secrets, are deleted. It delegates to the providers package to call the corresponding auth provider's cleanup method.
func (c Service) TriggerCleanup(config *v3.AuthConfig) error {
	err := providers.CleanupOnDisable(config)
	if err != nil {
		return fmt.Errorf("error cleaning up resources associated with a disabled auth provider: %w", err)
	}
	return nil
}
