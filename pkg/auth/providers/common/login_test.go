package common

import (
	"net/http"
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginConfigName(t *testing.T) {
	authConfig := func(name, configType string, enabled bool) map[string]any {
		return map[string]any{
			"metadata": map[string]any{"name": name},
			"type":     configType,
			"enabled":  enabled,
		}
	}

	tests := map[string]struct {
		configName  string
		configs     []map[string]any
		want        string
		wantStatus  int
		wantMessage string
	}{
		"configName of an enabled config of the provider": {
			configName: "github-eu",
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, true),
				authConfig("github-eu", client.GithubConfigType, true),
			},
			want: "github-eu",
		},
		"configName of a disabled config": {
			configName: "github-eu",
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, true),
				authConfig("github-eu", client.GithubConfigType, false),
			},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"configName of an unknown config": {
			configName: "github-eu",
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, true),
			},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"configName of a config of another provider": {
			configName: "openldap",
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, true),
				authConfig("openldap", client.OpenLdapConfigType, true),
			},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"the only enabled config of the provider": {
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, false),
				authConfig("github-eu", client.GithubConfigType, true),
				authConfig("openldap", client.OpenLdapConfigType, true),
			},
			want: "github-eu",
		},
		"no enabled config of the provider": {
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, false),
				authConfig("openldap", client.OpenLdapConfigType, true),
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "no auth config is enabled for provider github, specify configName",
		},
		"several enabled configs of the provider": {
			configs: []map[string]any{
				authConfig("github", client.GithubConfigType, true),
				authConfig("github-eu", client.GithubConfigType, true),
				authConfig("openldap", client.OpenLdapConfigType, true),
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "several auth configs are enabled for provider github, specify configName as one of: github, github-eu",
		},
		"several enabled configs of the provider are listed in order": {
			configs: []map[string]any{
				authConfig("github-eu", client.GithubConfigType, true),
				authConfig("github", client.GithubConfigType, true),
				authConfig("github-apac", client.GithubConfigType, true),
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "several auth configs are enabled for provider github, specify configName as one of: github, github-apac, github-eu",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			authConfigs := authconfigtest.NewLister(t, tt.configs...)

			got, err := LoginConfigName(authConfigs, tt.configName, "github")

			if tt.want == "" {
				authconfigtest.RequireLoginRejected(t, err, tt.wantStatus, tt.wantMessage)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
