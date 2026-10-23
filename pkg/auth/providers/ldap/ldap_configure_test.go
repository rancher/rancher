package ldap

import (
	"net/http"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigure verifies that Configure wires up the provider so that logins
// reach the AuthConfig lister under the provider's own name.
func TestConfigure(t *testing.T) {
	for _, providerName := range []string{OpenLdapName, FreeIpaName} {
		t.Run(providerName, func(t *testing.T) {
			p := Configure(authconfigtest.NewScaledContext(t), nil, nil, providerName)

			assert.Equal(t, providerName, p.GetName())
			provider, ok := p.(*ldapProvider)
			require.True(t, ok, "unexpected provider type %T", p)
			assert.NotNil(t, provider.authConfigLister, "authConfigLister is not set")

			// The lister's informer isn't started, so no config is enabled.
			_, _, _, err := p.AuthenticateUser(nil, nil, &apiv3.BasicLogin{
				GenericLogin: apiv3.GenericLogin{Name: providerName},
				Username:     "user",
				Password:     "password",
			})
			authconfigtest.RequireLoginRejected(t, err, http.StatusUnprocessableEntity,
				"no auth config is enabled for provider "+providerName+", specify configName")
		})
	}
}
