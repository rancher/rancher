package saml

import (
	"net/http"
	"net/http/httptest"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigure verifies that Configure wires up the provider so that logins
// reach the AuthConfig lister under the provider's own name.
func TestConfigure(t *testing.T) {
	for _, name := range []string{PingName, ADFSName, KeyCloakName, OKTAName, ShibbolethName, GenericSAMLName} {
		t.Run(name, func(t *testing.T) {
			setupSamlProviderTypes(t)

			p := Configure(t.Context(), authconfigtest.NewScaledContext(t), nil, nil, name)

			assert.Equal(t, name, p.GetName())
			provider, ok := p.(*Provider)
			require.True(t, ok, "unexpected provider type %T", p)
			assert.NotNil(t, provider.getSamlConfig, "getSamlConfig is not set")
			samlProvidersMu.RLock()
			assert.Same(t, provider, samlProviderTypes[name], "the provider is not registered under its name")
			samlProvidersMu.RUnlock()

			// The lister's informer isn't started, so no config is enabled.
			loginInput := &apiv3.SamlLoginInput{
				GenericLogin:     apiv3.GenericLogin{Name: name},
				FinalRedirectURL: "https://rancher.example.com/dashboard",
			}
			req := httptest.NewRequest(http.MethodPost, "https://rancher.example.com", nil)
			err := PerformSamlLogin(req, httptest.NewRecorder(), name, loginInput, p)
			authconfigtest.RequireLoginRejected(t, err, http.StatusUnprocessableEntity,
				"no auth config is enabled for provider "+name+", specify configName")
		})
	}
}
