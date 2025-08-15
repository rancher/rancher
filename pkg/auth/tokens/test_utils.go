package tokens

import (
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
)

// NewMockedManager returns a token manager with mocked clients to be used in other packages' tests.
func NewMockedManager(tokens v3.TokenClient) *Manager { // add other clients, indexers, listers as needed
	return &Manager{
		tokens: tokens,
	}
}
