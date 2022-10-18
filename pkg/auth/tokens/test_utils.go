package tokens

import (
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
)

// NewMockedManager returns a token manager with mocked clients to be used in other packages' tests.
func NewMockedManager(tokensClient *fakes.TokenInterfaceMock) *Manager { // add other clients, indexers, listers as needed
	return &Manager{
		tokensClient: tokensClient,
	}
}
