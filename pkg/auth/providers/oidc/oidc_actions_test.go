package oidc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/norman/api/writer"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

func Test_validScopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes string
		want   bool
	}{
		{
			name:   "valid single scope",
			scopes: "openid",
			want:   true,
		},
		{
			name:   "valid multiple scopes",
			scopes: "profile openid",
			want:   true,
		},
		{
			name: "no scopes",
		},
		{
			name:   "scopes lacking openid",
			scopes: "profile email",
		},
		{
			name:   "invalid scopes",
			scopes: "profile, email, openid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt := tt
			assert.Equalf(t, tt.want, validateScopes(tt.scopes), "validateScopes(%v)", tt.scopes)
		})
	}
}

// TestConfigureTest inspects the Redirect URL during the OIDC Provider setup.
func TestConfigureTest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		authConfig          map[string]any
		expectedRedirectURL string
	}{
		{
			name: "Setup of Generic Provider",
			authConfig: map[string]any{
				"accessMode":                "unrestricted",
				"authEndpoint":              "https://oidc.example.com/realms/testing/protocol/openid-connect/auth",
				"clientAuthenticatedSearch": false,
				"clientId":                  "rancher",
				"clientSecret":              "secretpassword",
				"enabled":                   false,
				"issuer":                    "https://oidc.example.com/realms/testing",
				"logoutAllEnabled":          false,
				"logoutAllForced":           false,
				"logoutAllSupported":        true,
				"name":                      "genericoidc",
				"rancherApiHost":            "https://localhost:9443",
				"rancherUrl":                "https://localhost:9443/verify-auth",
				"scope":                     "openid profile email testing",
				"type":                      "genericOIDCConfig",
			},
			// The scope is not URL-encoded this is done in the front-end when adding the nonce.
			expectedRedirectURL: "https://oidc.example.com/realms/testing/protocol/openid-connect/auth?scope=openid profile email testing&client_id=rancher&response_type=code&redirect_uri=https://localhost:9443/verify-auth",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := json.Marshal(test.authConfig)
			assert.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v3/genericoidcConfigs/genericoidc?action=configureTest", bytes.NewReader(b))

			schemas := types.NewSchemas()
			schemas.AddSchemas(managementschema.AuthSchemas)

			rw := &writer.EncodingResponseWriter{
				ContentType: "application/json",
				Encoder:     types.JSONEncoder,
			}
			rr := httptest.NewRecorder()
			r := &types.APIContext{
				Schemas:        schemas,
				Request:        req,
				Response:       rr,
				ResponseWriter: rw,
				Version:        &managementschema.Version,
			}

			provider := OpenIDCProvider{
				Type: "genericOIDCConfig",
			}
			err = provider.ConfigureTest(r)
			assert.NoError(t, err)

			res := rr.Result()
			defer res.Body.Close()

			var output v3.GenericOIDCTestOutput
			err = json.NewDecoder(res.Body).Decode(&output)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedRedirectURL, output.RedirectURL)
		})
	}

}
