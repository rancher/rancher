package oidc

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
	// keycloakoidc, genericoidc and cognito embed OpenIDCProvider and reuse its
	// login with Name set to their own provider name. Each gets a config of
	// another OIDC based type that it must not use.
	providers := map[string]struct {
		configType           string
		otherName, otherType string
	}{
		"oidc":         {configType: client.OIDCConfigType, otherName: "keycloakoidc", otherType: client.KeyCloakOIDCConfigType},
		"keycloakoidc": {configType: client.KeyCloakOIDCConfigType, otherName: "oidc", otherType: client.OIDCConfigType},
		"genericoidc":  {configType: client.GenericOIDCConfigType, otherName: "oidc", otherType: client.OIDCConfigType},
		"cognito":      {configType: client.CognitoConfigType, otherName: "genericoidc", otherType: client.GenericOIDCConfigType},
	}

	type testCase struct {
		configName string
		enabled    map[string]bool
		// wantLoaded is the config the login should load, or empty if the
		// login should be rejected.
		wantLoaded  string
		wantStatus  int
		wantMessage string
	}
	// tests returns the cases for provider p whose other type's config is
	// named other.
	tests := func(p, other string) map[string]testCase {
		return map[string]testCase{
			"type-only login uses the only enabled config": {
				enabled:    map[string]bool{p + "-corp": true, other: true},
				wantLoaded: p + "-corp",
			},
			"type-only login with no enabled config is rejected": {
				enabled:     map[string]bool{other: true},
				wantStatus:  http.StatusUnprocessableEntity,
				wantMessage: "no auth config is enabled for provider " + p + ", specify configName",
			},
			"type-only login with several enabled configs is rejected": {
				enabled:     map[string]bool{p: true, p + "-corp": true, other: true},
				wantStatus:  http.StatusUnprocessableEntity,
				wantMessage: "several auth configs are enabled for provider " + p + ", specify configName as one of: " + p + ", " + p + "-corp",
			},
			"login with configName uses that config": {
				configName: p + "-corp",
				enabled:    map[string]bool{p: true, p + "-corp": true},
				wantLoaded: p + "-corp",
			},
			"login with configName of a disabled config is rejected": {
				configName:  p,
				enabled:     map[string]bool{p + "-corp": true},
				wantStatus:  http.StatusUnauthorized,
				wantMessage: "authentication failed",
			},
			"login with configName of an unknown config is rejected": {
				configName:  p + "-missing",
				enabled:     map[string]bool{p: true},
				wantStatus:  http.StatusUnauthorized,
				wantMessage: "authentication failed",
			},
			"login with configName of a config of another type is rejected": {
				configName:  other,
				enabled:     map[string]bool{p: true, other: true},
				wantStatus:  http.StatusUnauthorized,
				wantMessage: "authentication failed",
			},
		}
	}

	for providerName, pc := range providers {
		for name, tt := range tests(providerName, pc.otherName) {
			t.Run(providerName+"/"+name, func(t *testing.T) {
				authConfigs, authConfigLister := authconfigtest.New(t,
					authConfig(providerName, pc.configType, tt.enabled[providerName]),
					authConfig(providerName+"-corp", pc.configType, tt.enabled[providerName+"-corp"]),
					authConfig(pc.otherName, pc.otherType, tt.enabled[pc.otherName]),
				)

				// Stop the login once the config is loaded, before it reaches
				// the identity provider.
				errStop := errors.New("stop after loading config")
				var loaded []string
				o := &OpenIDCProvider{
					Name:             providerName,
					AuthConfigs:      authConfigs,
					AuthConfigLister: authConfigLister,
					GetConfig: func(configName string) (*apiv3.OIDCConfig, error) {
						loaded = append(loaded, configName)
						return nil, errStop
					},
				}

				req := httptest.NewRequest(http.MethodPost, "https://rancher.example.com", nil)
				_, _, _, err := o.AuthenticateUser(httptest.NewRecorder(), req, &apiv3.OIDCLogin{
					GenericLogin: apiv3.GenericLogin{Name: providerName, ConfigName: tt.configName},
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
}

func authConfig(name, configType string, enabled bool) map[string]any {
	return map[string]any{
		"metadata": map[string]any{"name": name},
		"type":     configType,
		"enabled":  enabled,
	}
}
