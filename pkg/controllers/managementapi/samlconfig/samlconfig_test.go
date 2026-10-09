package samlconfig

import (
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/saml"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncRemovesSamlServiceProvider(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name         string
		config       *v3.AuthConfig
		shouldRemove bool
	}{
		{
			name: "missing config with unknown type",
		},
		{
			name:         "config being deleted",
			shouldRemove: true,
			config: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "okta-eu", DeletionTimestamp: &now},
				Type:       client.OKTAConfigType,
				Enabled:    true,
			},
		},
		{
			name: "non-SAML config being deleted",
			config: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "okta-eu", DeletionTimestamp: &now},
				Type:       client.GithubConfigType,
				Enabled:    true,
			},
		},
		{
			name:         "disabled config",
			shouldRemove: true,
			config: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "okta-eu"},
				Type:       client.OKTAConfigType,
				Enabled:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saml.SamlProviders["okta-eu"] = &saml.Provider{}
			t.Cleanup(func() {
				delete(saml.SamlProviders, "okta-eu")
			})

			a := &authProvider{}
			_, err := a.sync("okta-eu", tt.config)
			require.NoError(t, err)

			if tt.shouldRemove {
				assert.NotContains(t, saml.SamlProviders, "okta-eu")
			} else {
				assert.Contains(t, saml.SamlProviders, "okta-eu")
			}
		})
	}
}

func TestSyncIgnoresOtherConfigTypes(t *testing.T) {
	saml.SamlProviders["github-eu"] = &saml.Provider{}
	t.Cleanup(func() {
		delete(saml.SamlProviders, "github-eu")
	})

	a := &authProvider{}
	_, err := a.sync("github-eu", &v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "github-eu"},
		Type:       client.GithubConfigType,
		Enabled:    false,
	})
	require.NoError(t, err)

	assert.Contains(t, saml.SamlProviders, "github-eu", "only SAML configs should be removed when disabled")
}
