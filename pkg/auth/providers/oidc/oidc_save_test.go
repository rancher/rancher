package oidc

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSaveOIDCConfigUpdatesConfigWithSecretReferences(t *testing.T) {
	tests := map[string]struct {
		configName string
		wantPrefix string
	}{
		"default config": {
			configName: "genericoidc",
			wantPrefix: "genericoidcconfig",
		},
		"additional config": {
			configName: "test-eu-oidc",
			wantPrefix: "genericoidcconfig-test-eu-oidc",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := authconfigtest.NewUpdateServer(t, map[string]any{
				"metadata": map[string]any{"name": tt.configName},
				"type":     "genericOIDCConfig",
			})
			secrets, created := authconfigtest.NewSecretController(t)
			o := OpenIDCProvider{
				Name:        "genericoidc",
				Type:        "genericOIDCConfig",
				Secrets:     secrets,
				AuthConfigs: srv.Client(),
			}
			o.GetConfig = o.GetOIDCConfig

			config := &apiv3.OIDCConfig{
				AuthConfig:   apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: tt.configName}},
				PrivateKey:   "test-key",
				ClientSecret: "test-secret",
			}
			require.NoError(t, o.saveOIDCConfig(config))

			assert.Equal(t, 1, srv.Updates(), "the AuthConfig should be updated once")
			assert.Equal(t, map[string]map[string]string{
				tt.wantPrefix + "-privatekey":   {"privatekey": "test-key"},
				tt.wantPrefix + "-clientsecret": {"clientsecret": "test-secret"},
			}, created)
			stored := srv.Stored()
			assert.Equal(t, common.SecretsNamespace+":"+tt.wantPrefix+"-privatekey", stored["privateKey"])
			assert.Equal(t, common.SecretsNamespace+":"+tt.wantPrefix+"-clientsecret", stored["clientSecret"])
			assert.Equal(t, tt.configName, stored["metadata"].(map[string]any)["name"])
		})
	}
}

func TestSaveOIDCConfigRetriesOnConflict(t *testing.T) {
	srv := authconfigtest.NewUpdateServer(t, map[string]any{
		"metadata": map[string]any{"name": "genericoidc"},
		"type":     "genericOIDCConfig",
	})
	srv.WriteAfterGets(1)
	secrets, created := authconfigtest.NewSecretController(t)
	o := OpenIDCProvider{
		Name:        "genericoidc",
		Type:        "genericOIDCConfig",
		Secrets:     secrets,
		AuthConfigs: srv.Client(),
	}
	o.GetConfig = o.GetOIDCConfig

	config := &apiv3.OIDCConfig{
		AuthConfig:   apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericoidc"}},
		PrivateKey:   "test-key",
		ClientSecret: "test-secret",
	}
	require.NoError(t, o.saveOIDCConfig(config))

	authconfigtest.RequireUpdatedAfterConflict(t, srv)
	assert.Equal(t, map[string]map[string]string{
		"genericoidcconfig-privatekey":   {"privatekey": "test-key"},
		"genericoidcconfig-clientsecret": {"clientsecret": "test-secret"},
	}, created)
	assert.Equal(t, common.SecretsNamespace+":genericoidcconfig-privatekey", srv.Stored()["privateKey"])
	assert.Equal(t, common.SecretsNamespace+":genericoidcconfig-clientsecret", srv.Stored()["clientSecret"])
}
