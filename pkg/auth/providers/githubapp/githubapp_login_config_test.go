package githubapp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
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
			enabled:    map[string]bool{"githubapp-corp": true, "github": true},
			wantLoaded: "githubapp-corp",
		},
		"type-only login with no enabled config is rejected": {
			enabled:     map[string]bool{"github": true},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "no auth config is enabled for provider githubapp, specify configName",
		},
		"type-only login with several enabled configs is rejected": {
			enabled:     map[string]bool{"githubapp": true, "githubapp-corp": true, "github": true},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "several auth configs are enabled for provider githubapp, specify configName as one of: githubapp, githubapp-corp",
		},
		"login with configName uses that config": {
			configName: "githubapp-corp",
			enabled:    map[string]bool{"githubapp": true, "githubapp-corp": true},
			wantLoaded: "githubapp-corp",
		},
		"login with configName of a disabled config is rejected": {
			configName:  "githubapp",
			enabled:     map[string]bool{"githubapp-corp": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"login with configName of an unknown config is rejected": {
			configName:  "githubapp-missing",
			enabled:     map[string]bool{"githubapp": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"login with configName of a config of another type is rejected": {
			configName:  "github",
			enabled:     map[string]bool{"githubapp": true, "github": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			authConfigs, authConfigLister := authconfigtest.New(t,
				authConfig("githubapp", client.GithubAppConfigType, tt.enabled["githubapp"]),
				authConfig("githubapp-corp", client.GithubAppConfigType, tt.enabled["githubapp-corp"]),
				authConfig("github", client.GithubConfigType, tt.enabled["github"]),
			)

			// Stop the login once the config is loaded, before it reaches the
			// identity provider.
			errStop := errors.New("stop after loading config")
			var loaded []string
			p := &Provider{
				authConfigs:      authConfigs,
				authConfigLister: authConfigLister,
				getConfig: func(configName string) (*apiv3.GithubAppConfig, error) {
					loaded = append(loaded, configName)
					return nil, errStop
				},
			}

			req := httptest.NewRequest(http.MethodPost, "https://rancher.example.com", nil)
			_, _, _, err := p.AuthenticateUser(nil, req, &apiv3.GithubLogin{
				GenericLogin: apiv3.GenericLogin{Name: ProviderName, ConfigName: tt.configName},
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

func authConfig(name, configType string, enabled bool) map[string]any {
	return map[string]any{
		"metadata": map[string]any{"name": name},
		"type":     configType,
		"enabled":  enabled,
	}
}
