package providers

import (
	"fmt"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
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

func TestSearchPrincipalsHybridUser(t *testing.T) {
	users := []*apiv3.User{
		{
			// Hybrid user: kept a local login after OIDC was enabled. Both
			// principals have to reach the caller, since a binding on the OIDC
			// one grants nothing when they log in locally, and the other way
			// round.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-admin"},
			Username:     "admin",
			DisplayName:  "Default Admin",
			PrincipalIDs: []string{"genericoidc_user://sub-0009", "local://u-admin"},
		},
		{
			// OIDC user with no local login. Findable under genericoidc, and
			// must not surface as local. Issue rancher/rancher#54022.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc1"},
			DisplayName:  "Admin Assistant",
			PrincipalIDs: []string{"genericoidc_user://sub-0001", "local://u-oidc1"},
		},
	}

	lister := &fakes.UserListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
			return users, nil
		},
	}

	informer := cache.NewSharedIndexInformer(&cache.ListWatch{}, &apiv3.User{}, 0, cache.Indexers{})
	for _, u := range users {
		require.NoError(t, informer.GetIndexer().Add(u))
	}

	SetProviders(map[string]common.AuthProvider{
		genericoidc.Name: &genericoidc.GenOIDCProvider{
			OpenIDCProvider: oidc.OpenIDCProvider{
				Name:         genericoidc.Name,
				UserSearcher: common.NewUserSearcher(lister),
			},
		},
		local.Name: local.NewProvider(informer, lister, nil),
	})
	defer SetProviders(nil)

	got, err := SearchPrincipals("admin", "", &apiv3.Token{AuthProvider: genericoidc.Name})
	require.NoError(t, err)

	var ids []string
	for _, principal := range got {
		ids = append(ids, principal.Name)
	}

	assert.Equal(t, []string{
		"genericoidc_user://sub-0001", // Admin Assistant, resolved from the user cache
		"genericoidc_user://sub-0009", // hybrid user's real OIDC subject
		"genericoidc_user://admin",    // built from the search key
		"genericoidc_group://admin",   // same, for a group
		"local://u-admin",             // hybrid user's local login
	}, ids)
}
