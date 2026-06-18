package providers

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/features"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestIsSAMLProvider(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"ping", "pingConfig", "pingProvider",
		"adfs", "adfsConfig", "adfsProvider",
		"keycloak", "keycloakConfig", "keycloakProvider",
		"okta", "oktaConfig", "oktaProvider",
		"shibboleth", "shibbolethConfig", "shibbolethProvider",
	} {
		assert.True(t, IsSAMLProviderType(name), name)
	}

	for _, name := range []string{
		"github", "githubConfig", "githubProvider",
		"local", "localConfig", "localProvider",
	} {
		assert.False(t, IsSAMLProviderType(name), name)
	}
}

func TestProviderUsesUserSecrets(t *testing.T) {
	SetProviders(map[string]common.AuthProvider{
		github.Name:    &github.Provider{},
		githubapp.Name: &githubapp.Provider{},
		local.Name:     &local.Provider{},
	})
	defer SetProviders(nil)

	assert.True(t, ProviderUsesUserSecrets(github.Name))
	assert.False(t, ProviderUsesUserSecrets(githubapp.Name))
	assert.False(t, ProviderUsesUserSecrets(local.Name))
}

func TestProviderCanRefreshPrincipals(t *testing.T) {
	SetProviders(map[string]common.AuthProvider{
		github.Name:      &github.Provider{},
		genericoidc.Name: &genericoidc.GenOIDCProvider{},
	})
	defer SetProviders(nil)

	assert.True(t, ProviderCanRefreshPrincipals(github.Name))
	assert.False(t, ProviderCanRefreshPrincipals(genericoidc.Name))
}

func TestIsExternalProviderEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)

	active := mocks.NewMockAuthProvider(ctrl)
	active.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()

	inactive := mocks.NewMockAuthProvider(ctrl)
	inactive.EXPECT().IsDisabledProvider().Return(true, nil).AnyTimes()

	broken := mocks.NewMockAuthProvider(ctrl)
	broken.EXPECT().IsDisabledProvider().Return(false, fmt.Errorf("db timeout")).AnyTimes()

	tests := []struct {
		name     string
		registry map[string]common.AuthProvider
		want     bool
	}{
		{
			name:     "only local registered",
			registry: map[string]common.AuthProvider{local.Name: &local.Provider{}},
			want:     false,
		},
		{
			name: "external provider disabled",
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
				"github":   inactive,
			},
			want: false,
		},
		{
			name: "external provider enabled",
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
				"github":   active,
			},
			want: true,
		},
		{
			// When a provider cannot be reached, the safe default is to treat it as
			// not enabled and keep local auth visible. Hiding local auth when external
			// auth is unreachable would lock admins out of the cluster.
			name: "unreachable provider keeps local visible",
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
				"github":   broken,
			},
			want: false,
		},
		{
			name: "one broken one active returns true",
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
				"github":   broken,
				"okta":     active,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetProviders(tt.registry)
			defer SetProviders(nil)
			assert.Equal(t, tt.want, IsExternalProviderEnabled())
		})
	}
}

func TestIsLocalHidden(t *testing.T) {
	ctrl := gomock.NewController(t)

	active := mocks.NewMockAuthProvider(ctrl)
	active.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()

	tests := []struct {
		name     string
		flag     bool
		registry map[string]common.AuthProvider
		want     bool
	}{
		{
			name: "feature flag off skips provider check",
			flag: false,
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
				"github":   active,
			},
			want: false,
		},
		{
			name: "feature flag on no external provider",
			flag: true,
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
			},
			want: false,
		},
		{
			name: "feature flag on external provider active",
			flag: true,
			registry: map[string]common.AuthProvider{
				local.Name: &local.Provider{},
				"github":   active,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features.HideLocalAuthProvider.Set(tt.flag)
			defer features.HideLocalAuthProvider.Set(false)
			SetProviders(tt.registry)
			defer SetProviders(nil)
			assert.Equal(t, tt.want, IsLocalHidden())
		})
	}
}

func TestIsExternalProviderEnabledFastPathErrorRetriedInFullScan(t *testing.T) {
	ctrl := gomock.NewController(t)

	provider := mocks.NewMockAuthProvider(ctrl)
	gomock.InOrder(
		provider.EXPECT().IsDisabledProvider().Return(false, nil),                           // full scan warms the hint
		provider.EXPECT().IsDisabledProvider().Return(false, fmt.Errorf("transient error")), // fast path errors: alreadyChecked must stay ""
		provider.EXPECT().IsDisabledProvider().Return(false, nil),                           // full scan retries and confirms enabled
	)

	SetProviders(map[string]common.AuthProvider{
		local.Name: &local.Provider{},
		"github":   provider,
	})
	defer SetProviders(nil)

	assert.True(t, IsExternalProviderEnabled(), "first call: full scan finds provider enabled and warms hint")
	assert.True(t, IsExternalProviderEnabled(), "second call: fast-path error must not prevent full scan from retrying the provider")
}

func TestIsLocalHiddenReflectsProviderStateChange(t *testing.T) {
	ctrl := gomock.NewController(t)

	provider := mocks.NewMockAuthProvider(ctrl)
	gomock.InOrder(
		provider.EXPECT().IsDisabledProvider().Return(false, nil), // external enabled
		provider.EXPECT().IsDisabledProvider().Return(true, nil),  // external disabled
	)

	features.HideLocalAuthProvider.Set(true)
	defer features.HideLocalAuthProvider.Set(false)
	SetProviders(map[string]common.AuthProvider{
		local.Name: &local.Provider{},
		"github":   provider,
	})
	defer SetProviders(nil)

	assert.True(t, IsLocalHidden(), "local should be hidden while external provider is active")
	assert.False(t, IsLocalHidden(), "local should reappear after external provider is disabled")
}
