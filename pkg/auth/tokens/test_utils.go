package tokens

import (
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ctrlv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

// NewMockedManager returns a token manager with mocked clients to be used in other packages' tests.
func NewMockedManager(tokens v3.TokenClient, secretCache ctrlv1.SecretCache) *Manager {
	return &Manager{
		tokens:      tokens,
		secretCache: secretCache,
	}
}
