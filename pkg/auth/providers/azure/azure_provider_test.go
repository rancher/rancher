package azure

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestConfigureTest inspects the Redirect URL during Azure AD setup.
func TestConfigureTest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		authConfig          map[string]any
		expectedRedirectURL string
	}{
		{
			name: "initial setup of Azure AD with Microsoft Graph",
			authConfig: map[string]any{
				"accessMode": "unrestricted",
				"annotations": map[string]any{
					"auth.cattle.io/azuread-endpoint-migrated": "true",
				},
				"enabled":           false,
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.microsoft.com",
				"tokenEndpoint":     "https://login.microsoftonline.com/tenant123/oauth2/v2.0/token",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedRedirectURL: "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
		},
		{
			name: "attempt to initially setup Azure AD with deprecated Azure AD Graph",
			authConfig: map[string]any{
				"accessMode":        "unrestricted",
				"annotations":       map[string]any{},
				"enabled":           false,
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.windows.net/",
				"tokenEndpoint":     "https://login.microsoftonline.com/tenant123/oauth2/token",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedRedirectURL: "https://login.microsoftonline.com/tenant123/oauth2/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
		},
		{
			name: "editing an existing setup of Azure AD",
			authConfig: map[string]any{
				"enabled":    true,
				"accessMode": "unrestricted",
				"annotations": map[string]any{
					"auth.cattle.io/azuread-endpoint-migrated": "true",
				},
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.microsoft.com",
				"tokenEndpoint":     "https://login.microsoftonline.com/tenant123/oauth2/v2.0/token",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedRedirectURL: "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := json.Marshal(test.authConfig)
			assert.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v3/azureADConfigs/azuread?action=configureTest", bytes.NewReader(b))

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

			provider := Provider{}
			err = provider.ConfigureTest(r)
			assert.NoError(t, err)

			res := rr.Result()
			defer res.Body.Close()

			var output v3.AzureADConfigTestOutput
			err = json.NewDecoder(res.Body).Decode(&output)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedRedirectURL, output.RedirectURL)
		})
	}

}

func TestTransformToAuthProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		authConfig           map[string]any
		expectedAuthProvider map[string]any
	}{
		{
			name: "redirect URL for Microsoft Graph",
			authConfig: map[string]any{
				"enabled":    true,
				"accessMode": "unrestricted",
				"metadata": map[string]any{
					"name": "providerName",
					"annotations": map[string]any{
						"auth.cattle.io/azuread-endpoint-migrated": "true",
					},
				},
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.microsoft.com",
				"tokenEndpoint":     "https://login.microsoftonline.com/tenant123/oauth2/v2.0/token",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedAuthProvider: map[string]any{
				"id":                 "providerName",
				"clientId":           "app123",
				"tenantId":           "tenant123",
				"scopes":             []string{"openid", "profile", "email"},
				"authUrl":            "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize",
				"tokenUrl":           "https://login.microsoftonline.com/tenant123/oauth2/v2.0/token",
				"deviceAuthUrl":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/devicecode",
				"redirectUrl":        "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			name: "redirect URL for disabled auth provider with annotation",
			authConfig: map[string]any{
				"accessMode": "unrestricted",
				"metadata": map[string]any{
					"name": "providerName",
					"annotations": map[string]any{
						"auth.cattle.io/azuread-endpoint-migrated": "true",
					},
				},
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.microsoft.com",
				"tokenEndpoint":     "https://login.microsoftonline.com/tenant123/oauth2/token",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedAuthProvider: map[string]any{
				"id":                 "providerName",
				"clientId":           "app123",
				"tenantId":           "tenant123",
				"scopes":             []string{"openid", "profile", "email"},
				"authUrl":            "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize",
				"tokenUrl":           "https://login.microsoftonline.com/tenant123/oauth2/token",
				"deviceAuthUrl":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/devicecode",
				"redirectUrl":        "https://login.microsoftonline.com/tenant123/oauth2/v2.0/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			name: "redirect URL for disabled auth provider without annotation",
			authConfig: map[string]any{
				"enabled":    false, // Here, enabled is set to false explicitly.
				"accessMode": "unrestricted",
				"metadata": map[string]any{
					"name":        "providerName",
					"annotations": map[string]any{},
				},
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.windows.net/",
				"tokenEndpoint":     "https://login.microsoftonline.com/tenant123/oauth2/token",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedAuthProvider: map[string]any{
				"id":                 "providerName",
				"clientId":           "app123",
				"tenantId":           "tenant123",
				"scopes":             []string{"openid", "profile", "email"},
				"authUrl":            "https://login.microsoftonline.com/tenant123/oauth2/authorize",
				"tokenUrl":           "https://login.microsoftonline.com/tenant123/oauth2/token",
				"deviceAuthUrl":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/devicecode",
				"redirectUrl":        "https://login.microsoftonline.com/tenant123/oauth2/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			name: "oauth URLs from default endpoint",
			authConfig: map[string]any{
				"enabled":    false, // Here, enabled is set to false explicitly.
				"accessMode": "unrestricted",
				"metadata": map[string]any{
					"name":        "providerName",
					"annotations": map[string]any{},
				},
				"endpoint":          "https://login.microsoftonline.com/",
				"graphEndpoint":     "https://graph.windows.net/",
				"authEndpoint":      "https://login.microsoftonline.com/tenant123/oauth2/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedAuthProvider: map[string]any{
				"id":                 "providerName",
				"clientId":           "app123",
				"tenantId":           "tenant123",
				"scopes":             []string{"openid", "profile", "email"},
				"authUrl":            "https://login.microsoftonline.com/tenant123/oauth2/authorize",
				"tokenUrl":           "https://login.microsoftonline.com/tenant123/oauth2/v2.0/token",
				"deviceAuthUrl":      "https://login.microsoftonline.com/tenant123/oauth2/v2.0/devicecode",
				"redirectUrl":        "https://login.microsoftonline.com/tenant123/oauth2/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			name: "oauth URLs from custom endpoint and no oauth URLs",
			authConfig: map[string]any{
				"enabled":    false, // Here, enabled is set to false explicitly.
				"accessMode": "unrestricted",
				"metadata": map[string]any{
					"name":        "providerName",
					"annotations": map[string]any{},
				},
				"endpoint":          "https://myendpoint.com/",
				"graphEndpoint":     "https://graph.windows.net/",
				"authEndpoint":      "https://myendpoint.com/tenant123/oauth2/authorize",
				"tenantId":          "tenant123",
				"applicationId":     "app123",
				"applicationSecret": "secret123",
				"rancherUrl":        "https://myrancher.com",
			},
			expectedAuthProvider: map[string]any{
				"id":                 "providerName",
				"clientId":           "app123",
				"tenantId":           "tenant123",
				"scopes":             []string{"openid", "profile", "email"},
				"authUrl":            "https://myendpoint.com/tenant123/oauth2/authorize",
				"tokenUrl":           "https://myendpoint.com/tenant123/oauth2/v2.0/token",
				"deviceAuthUrl":      "https://myendpoint.com/tenant123/oauth2/v2.0/devicecode",
				"redirectUrl":        "https://myendpoint.com/tenant123/oauth2/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			name: "oauth URLs from custom URLs",
			authConfig: map[string]any{
				"enabled":    false, // Here, enabled is set to false explicitly.
				"accessMode": "unrestricted",
				"metadata": map[string]any{
					"name":        "providerName",
					"annotations": map[string]any{},
				},
				"endpoint":           "https://login.microsoftonline.com/",
				"graphEndpoint":      "https://graph.windows.net/",
				"tokenEndpoint":      "https://custom.com/oauth2/token",
				"authEndpoint":       "https://custom.com/oauth2/authorize",
				"deviceAuthEndpoint": "https://custom.com/oauth2/device",
				"tenantId":           "tenant123",
				"applicationId":      "app123",
				"applicationSecret":  "secret123",
				"rancherUrl":         "https://myrancher.com",
			},
			expectedAuthProvider: map[string]any{
				"id":                 "providerName",
				"clientId":           "app123",
				"tenantId":           "tenant123",
				"scopes":             []string{"openid", "profile", "email"},
				"authUrl":            "https://custom.com/oauth2/authorize",
				"tokenUrl":           "https://custom.com/oauth2/token",
				"deviceAuthUrl":      "https://custom.com/oauth2/device",
				"redirectUrl":        "https://custom.com/oauth2/authorize?client_id=app123&redirect_uri=https://myrancher.com&response_type=code&scope=openid",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
	}

	var provider Provider
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authProvider, err := provider.TransformToAuthProvider(test.authConfig)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedAuthProvider, authProvider)
		})
	}
}

func TestMigrateNewFlowAnnotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		current            *v3.AzureADConfig
		proposed           *v3.AzureADConfig
		annotationExpected bool
	}{
		{
			name: "new setup on Rancher v2.6.7+ after an upgrade from previous version",
			current: &v3.AzureADConfig{
				AuthConfig: v3.AuthConfig{
					Enabled: false,
				},
				GraphEndpoint: "https://graph.microsoft.com",
			},
			proposed:           &v3.AzureADConfig{},
			annotationExpected: true,
		},
		{
			name: "new setup on Rancher v2.6.7+",
			current: &v3.AzureADConfig{
				AuthConfig: v3.AuthConfig{
					Enabled: false,
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							GraphEndpointMigratedAnnotation: "true",
						},
					},
				},
				GraphEndpoint: "https://graph.microsoft.com",
			},
			proposed:           &v3.AzureADConfig{},
			annotationExpected: true,
		},
		{
			name: "reconfigure existing new setup",
			current: &v3.AzureADConfig{
				AuthConfig: v3.AuthConfig{
					Enabled: true,
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							GraphEndpointMigratedAnnotation: "true",
						},
					},
				},
				GraphEndpoint: "https://graph.microsoft.com",
			},
			proposed:           &v3.AzureADConfig{},
			annotationExpected: true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			migrateNewFlowAnnotation(test.current, test.proposed)
			_, hasAnnotation := test.proposed.Annotations[GraphEndpointMigratedAnnotation]
			if test.annotationExpected && !hasAnnotation {
				assert.Fail(t, "expected annotation on the processed config, but did not find one")
			}
			if !test.annotationExpected && hasAnnotation {
				assert.Fail(t, "did not expect the annotation on the processed config, but found one")
			}
		})
	}
}
