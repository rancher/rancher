package genericoidc

import (
	"context"
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
	p := Configure(context.Background(), authconfigtest.NewScaledContext(t), nil, nil)

	assert.Equal(t, ProviderName, p.GetName())
	provider, ok := p.(*GenOIDCProvider)
	require.True(t, ok, "unexpected provider type %T", p)
	assert.NotNil(t, provider.GetConfig, "GetConfig is not set")

	// The lister's informer isn't started, so no config is enabled.
	req := httptest.NewRequest(http.MethodPost, "https://rancher.example.com", nil)
	_, _, _, err := p.AuthenticateUser(httptest.NewRecorder(), req, &apiv3.OIDCLogin{GenericLogin: apiv3.GenericLogin{Name: ProviderName}, Code: "code"})
	authconfigtest.RequireLoginRejected(t, err, http.StatusUnprocessableEntity,
		"no auth config is enabled for provider "+ProviderName+", specify configName")
}
