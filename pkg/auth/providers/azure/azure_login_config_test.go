package azure

import (
	"errors"
	"net/http"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticateUserResolvesConfigName(t *testing.T) {
	tests := map[string]struct {
		configName string
		enabled    map[string]bool
		// wantLoaded is the config the login should load, or empty if the
		// login should be rejected.
		wantLoaded  string
		wantStatus  int
		wantMessage string
	}{
		"type-only login uses the only enabled config": {
			enabled:    map[string]bool{"azuread-corp": true, "activedirectory": true},
			wantLoaded: "azuread-corp",
		},
		"type-only login with no enabled config is rejected": {
			enabled:     map[string]bool{"activedirectory": true},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "no auth config is enabled for provider azuread, specify configName",
		},
		"type-only login with several enabled configs is rejected": {
			enabled:     map[string]bool{"azuread": true, "azuread-corp": true, "activedirectory": true},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "several auth configs are enabled for provider azuread, specify configName as one of: azuread, azuread-corp",
		},
		"login with configName uses that config": {
			configName: "azuread-corp",
			enabled:    map[string]bool{"azuread": true, "azuread-corp": true},
			wantLoaded: "azuread-corp",
		},
		"login with configName of a disabled config is rejected": {
			configName:  "azuread",
			enabled:     map[string]bool{"azuread-corp": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"login with configName of an unknown config is rejected": {
			configName:  "azuread-missing",
			enabled:     map[string]bool{"azuread": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"login with configName of a config of another provider is rejected": {
			configName:  "activedirectory",
			enabled:     map[string]bool{"azuread": true, "activedirectory": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			authConfigs, authConfigLister := authconfigtest.New(t,
				azureAuthConfig("azuread", client.AzureADConfigType, tt.enabled["azuread"]),
				azureAuthConfig("azuread-corp", client.AzureADConfigType, tt.enabled["azuread-corp"]),
				azureAuthConfig("activedirectory", client.ActiveDirectoryConfigType, tt.enabled["activedirectory"]),
			)

			// Stop the login once the config is loaded, before it reaches Azure.
			errStop := errors.New("stop after loading config")
			var loaded []string
			p := &Provider{
				authConfigs:      authConfigs,
				authConfigLister: authConfigLister,
				Retriever:        authConfigs.ObjectClient().UnstructuredClient(),
				getConfig: func(configName string) (*apiv3.AzureADConfig, error) {
					loaded = append(loaded, configName)
					return nil, errStop
				},
			}

			_, _, _, err := p.AuthenticateUser(nil, nil, &apiv3.AzureADLogin{
				GenericLogin: apiv3.GenericLogin{Type: publicclient.AzureADProviderType, Name: ProviderName, ConfigName: tt.configName},
				Code:         "code",
			})

			if tt.wantLoaded == "" {
				authconfigtest.RequireLoginRejected(t, err, tt.wantStatus, tt.wantMessage)
				assert.Empty(t, loaded)
				return
			}

			require.ErrorIs(t, err, errStop)
			assert.Equal(t, []string{tt.wantLoaded}, loaded)
		})
	}
}

func azureAuthConfig(name, configType string, enabled bool) map[string]any {
	return map[string]any{
		"metadata": map[string]any{"name": name},
		"type":     configType,
		"enabled":  enabled,
	}
}
