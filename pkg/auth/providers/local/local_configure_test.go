package local

import (
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/rancher/rancher/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUserManager embeds user.Manager so that only the methods a test needs
// have to be implemented; calling any other method panics.
type fakeUserManager struct {
	user.Manager
}

// TestConfigure verifies that Configure sets the AuthConfig lister, without
// which principals of AuthConfigs named unlike their provider can't be
// resolved, and the user manager that GetKnownPrincipal looks users up with.
func TestConfigure(t *testing.T) {
	userMGR := fakeUserManager{}
	p := Configure(t.Context(), authconfigtest.NewScaledContext(t), userMGR)

	assert.Equal(t, Name, p.GetName())
	provider, ok := p.(*Provider)
	require.True(t, ok, "unexpected provider type %T", p)
	assert.NotNil(t, provider.authConfigLister, "authConfigLister is not set")
	assert.Equal(t, userMGR, provider.users, "users is not set")
}
