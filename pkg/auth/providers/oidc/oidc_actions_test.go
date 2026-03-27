package oidc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/norman/api/writer"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func TestConfigFromApplyInput(t *testing.T) {
	tests := map[string]struct {
		input    apiv3.OIDCApplyInput
		wantName string
	}{
		"configName is used": {
			input: apiv3.OIDCApplyInput{
				ConfigName: "test-eu-oidc",
				OIDCConfig: apiv3.OIDCConfig{AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "other"}}},
			},
			wantName: "test-eu-oidc",
		},
		"missing configName falls back to the config name": {
			input: apiv3.OIDCApplyInput{
				OIDCConfig: apiv3.OIDCConfig{AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "test-eu-oidc"}}},
			},
			wantName: "test-eu-oidc",
		},
		"missing configName and config name falls back to the provider name": {
			input:    apiv3.OIDCApplyInput{},
			wantName: ProviderName,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			o := &OpenIDCProvider{Name: ProviderName}
			tt.input.Code = "test-code"

			config, login := o.configFromApplyInput(&tt.input)

			assert.Equal(t, tt.wantName, config.Name)
			assert.Equal(t, tt.wantName, login.ConfigName)
			assert.Equal(t, "test-code", login.Code)
			if assert.NotNil(t, config.GroupSearchEnabled) {
				assert.False(t, *config.GroupSearchEnabled)
			}
		})
	}
}

func TestConfigFromApplyInputCognito(t *testing.T) {
	o := &OpenIDCProvider{Name: "cognito"}
	input := &apiv3.OIDCApplyInput{
		ConfigName: "test-eu-cognito",
		OIDCConfig: apiv3.OIDCConfig{AuthConfig: apiv3.AuthConfig{Type: client.CognitoConfigType}},
	}

	config, _ := o.configFromApplyInput(input)

	assert.Equal(t, cognitoGroupsClaim, config.GroupsClaim)
}
