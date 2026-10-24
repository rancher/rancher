package providers

import (
	"fmt"
	"testing"

	"github.com/rancher/norman/httperror"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func TestGenericSAMLTypeMapsToProviderName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "genericsaml", NameFromType(client.GenericSAMLConfigType))
	assert.Equal(t, "genericSAMLConfig", client.GenericSAMLConfigType)
}

func TestGenericSAMLIsSAMLProviderType(t *testing.T) {
	assert.True(t, IsSAMLProviderType(client.GenericSAMLConfigType))
}

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
		github.ProviderName:    &github.Provider{},
		githubapp.ProviderName: &githubapp.Provider{},
		local.Name:             &local.Provider{},
	})
	defer SetProviders(nil)

	assert.True(t, ProviderUsesUserSecrets(github.ProviderName))
	assert.False(t, ProviderUsesUserSecrets(githubapp.ProviderName))
	assert.False(t, ProviderUsesUserSecrets(local.Name))
}

func TestProviderCanRefreshPrincipals(t *testing.T) {
	SetProviders(map[string]common.AuthProvider{
		github.ProviderName:      &github.Provider{},
		genericoidc.ProviderName: &genericoidc.GenOIDCProvider{},
	})
	defer SetProviders(nil)

	assert.True(t, ProviderCanRefreshPrincipals(github.ProviderName))
	assert.False(t, ProviderCanRefreshPrincipals(genericoidc.ProviderName))
}

func TestIsExternalProviderEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)

	active := mocks.NewMockAuthProvider(ctrl)
	active.EXPECT().GetName().Return(local.Name).AnyTimes()
	active.EXPECT().IsDisabledProvider(local.Name).Return(false, nil).AnyTimes()

	inactive := mocks.NewMockAuthProvider(ctrl)
	inactive.EXPECT().GetName().Return(github.ProviderName).AnyTimes()
	inactive.EXPECT().IsDisabledProvider(github.ProviderName).Return(true, nil).AnyTimes()

	broken := mocks.NewMockAuthProvider(ctrl)
	broken.EXPECT().GetName().Return(github.ProviderName).AnyTimes()
	broken.EXPECT().IsDisabledProvider("github").Return(false, fmt.Errorf("db timeout")).AnyTimes()

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
	active.EXPECT().GetName().Return("github").AnyTimes()
	active.EXPECT().IsDisabledProvider("github").Return(false, nil).AnyTimes()

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
	provider.EXPECT().GetName().Return("github").AnyTimes()
	gomock.InOrder(
		provider.EXPECT().IsDisabledProvider("github").Return(false, nil),                           // full scan warms the hint
		provider.EXPECT().IsDisabledProvider("github").Return(false, fmt.Errorf("transient error")), // fast path errors: alreadyChecked must stay ""
		provider.EXPECT().IsDisabledProvider("github").Return(false, nil),                           // full scan retries and confirms enabled
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
	provider.EXPECT().GetName().Return("github").AnyTimes()
	gomock.InOrder(
		provider.EXPECT().IsDisabledProvider("github").Return(false, nil), // external enabled
		provider.EXPECT().IsDisabledProvider("github").Return(true, nil),  // external disabled
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
		genericoidc.ProviderName: &genericoidc.GenOIDCProvider{
			OpenIDCProvider: oidc.OpenIDCProvider{
				Name:         genericoidc.ProviderName,
				UserSearcher: common.NewUserSearcher(lister),
			},
		},
		local.Name: local.NewProvider(informer, lister, nil),
	})
	defer SetProviders(nil)

	token := &apiv3.Token{
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "genericoidc_user://9253000",
			},
			LoginName:     "developer",
			PrincipalType: "user",
		},
	}

	got, err := SearchPrincipals("admin", "", token)
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

func TestIsExternalProviderEnabledChecksEveryAuthConfig(t *testing.T) {
	ctrl := gomock.NewController(t)

	provider := mocks.NewMockAuthProvider(ctrl)
	provider.EXPECT().IsDisabledProvider("github").Return(true, nil).AnyTimes()
	provider.EXPECT().IsDisabledProvider("github-eu").Return(false, nil).AnyTimes()

	SetProviders(map[string]common.AuthProvider{
		local.Name: &local.Provider{},
		"github":   provider,
	})
	defer SetProviders(nil)

	newAuthConfig := func(name, configType string) *apiv3.AuthConfig {
		return &apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: configType}
	}
	SetAuthConfigLister(&fakes.AuthConfigListerMock{
		ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.AuthConfig, error) {
			return []*apiv3.AuthConfig{
				newAuthConfig(local.Name, client.LocalConfigType),
				newAuthConfig("github", client.GithubConfigType),
				newAuthConfig("github-eu", client.GithubConfigType),
			}, nil
		},
	})
	defer SetAuthConfigLister(nil)

	assert.True(t, IsExternalProviderEnabled(), "an enabled named config should count as an enabled external provider")
	assert.True(t, IsExternalProviderEnabled(), "the fast path should confirm the named config")
}

func TestProviderNameForConfig(t *testing.T) {
	ctrl := gomock.NewController(t)

	githubProvider := mocks.NewMockAuthProvider(ctrl)
	githubProvider.EXPECT().GetName().Return(github.ProviderName).AnyTimes()

	SetProviders(map[string]common.AuthProvider{
		local.Name:          &local.Provider{},
		github.ProviderName: githubProvider,
	})
	t.Cleanup(func() { SetProviders(nil) })

	t.Run("without a lister configs are named after their provider", func(t *testing.T) {
		name, err := ProviderNameForConfig(github.ProviderName)
		require.NoError(t, err)
		assert.Equal(t, github.ProviderName, name)

		name, err = ProviderNameForConfig(local.Name)
		require.NoError(t, err)
		assert.Equal(t, local.Name, name)

		_, err = ProviderNameForConfig("github-eu")
		assert.ErrorContains(t, err, "no such auth config 'github-eu'")
	})

	t.Run("with a lister configs are looked up by name", func(t *testing.T) {
		authConfigs := map[string]*apiv3.AuthConfig{
			"github-eu":       {ObjectMeta: metav1.ObjectMeta{Name: "github-eu"}, Type: client.GithubConfigType},
			"corp-ad":         {ObjectMeta: metav1.ObjectMeta{Name: "corp-ad"}, Type: client.ActiveDirectoryConfigType},
			"activedirectory": {ObjectMeta: metav1.ObjectMeta{Name: "activedirectory"}, Type: client.ActiveDirectoryConfigType},
		}
		SetAuthConfigLister(&fakes.AuthConfigListerMock{
			GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
				if authConfig, ok := authConfigs[name]; ok {
					return authConfig, nil
				}
				return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
			},
		})
		t.Cleanup(func() { SetAuthConfigLister(nil) })

		for configName, want := range map[string]string{
			"github-eu":       github.ProviderName,
			"corp-ad":         "activedirectory",
			"activedirectory": "activedirectory",
			// The local config is never looked up.
			local.Name: local.Name,
		} {
			name, err := ProviderNameForConfig(configName)
			require.NoError(t, err, configName)
			assert.Equal(t, want, name, configName)
		}

		_, err := ProviderNameForConfig(github.ProviderName)
		assert.True(t, apierrors.IsNotFound(err), "expected a not found error, got %v", err)
	})
}

func TestGetPrincipalUnknownAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	localProvider := mocks.NewMockAuthProvider(ctrl)
	SetProviders(map[string]common.AuthProvider{local.Name: localProvider})
	t.Cleanup(func() { SetProviders(nil) })
	SetAuthConfigLister(&fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
			if name == "github-eu" {
				return &apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.GithubConfigType}, nil
			}
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
		},
	})
	t.Cleanup(func() { SetAuthConfigLister(nil) })

	newToken := func(principalID string) *apiv3.Token {
		// The deprecated auth provider is ignored.
		return &apiv3.Token{
			AuthProvider:  local.Name,
			UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: principalID}},
		}
	}

	t.Run("auth config doesn't exist", func(t *testing.T) {
		_, err := GetPrincipal("local://u-abc", newToken("unknown_user://1"))
		assert.True(t, apierrors.IsNotFound(err), "expected a not found error, got %v", err)
	})

	t.Run("provider of the auth config isn't registered", func(t *testing.T) {
		_, err := GetPrincipal("local://u-abc", newToken("github-eu_user://1"))
		require.ErrorContains(t, err, `authProvider "github" not initialized`)
	})

	t.Run("search with a token whose auth config doesn't exist", func(t *testing.T) {
		// The local provider mock expects no calls: the search must not fall
		// back to it.
		got, err := SearchPrincipals("alice", "", newToken("unknown_user://1"))
		assert.True(t, apierrors.IsNotFound(err), "expected a not found error, got %v", err)
		assert.Empty(t, got)
	})
}

func TestProviderNameForToken(t *testing.T) {
	SetProviders(map[string]common.AuthProvider{local.Name: &local.Provider{}})
	t.Cleanup(func() { SetProviders(nil) })
	SetAuthConfigLister(&fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
			if name == "github-eu" {
				return &apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.GithubConfigType}, nil
			}
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
		},
	})
	t.Cleanup(func() { SetAuthConfigLister(nil) })

	for name, tt := range map[string]struct {
		token *apiv3.Token
		want  string
	}{
		"token from an auth config": {
			// The deprecated auth provider is ignored.
			token: &apiv3.Token{AuthProvider: "other", UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}}},
			want:  github.ProviderName,
		},
		"token with no principal belongs to local": {
			token: &apiv3.Token{AuthProvider: github.ProviderName},
			want:  local.Name,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ProviderNameForToken(tt.token)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("token with an unparseable principal", func(t *testing.T) {
		token := &apiv3.Token{UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "not-a-principal"}}}
		_, err := ProviderNameForToken(token)
		assert.EqualError(t, err, "invalid principal: not-a-principal")
	})
}

func TestIsConfigDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	githubProvider := mocks.NewMockAuthProvider(ctrl)
	githubProvider.EXPECT().IsDisabledProvider("github-eu").Return(false, nil).AnyTimes()
	githubProvider.EXPECT().IsDisabledProvider("github-off").Return(true, nil).AnyTimes()
	githubProvider.EXPECT().IsDisabledProvider("github-err").Return(false, fmt.Errorf("can't get auth config")).AnyTimes()
	SetProviders(map[string]common.AuthProvider{github.ProviderName: githubProvider})
	t.Cleanup(func() { SetProviders(nil) })
	SetAuthConfigLister(&fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
			switch name {
			case "github-eu", "github-off", "github-err":
				return &apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.GithubConfigType}, nil
			}
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
		},
	})
	t.Cleanup(func() { SetAuthConfigLister(nil) })

	for name, tt := range map[string]struct {
		configName   string
		wantDisabled bool
		wantErr      string
	}{
		"enabled config": {
			configName: "github-eu",
		},
		"disabled config": {
			configName:   "github-off",
			wantDisabled: true,
		},
		"missing config": {
			configName: "github-missing",
			wantErr:    `getting auth config 'github-missing': authconfig.management.cattle.io "github-missing" not found`,
		},
		"config that can't be checked": {
			configName: "github-err",
			wantErr:    "can't get auth config",
		},
	} {
		t.Run(name, func(t *testing.T) {
			disabled, err := IsConfigDisabled(tt.configName)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDisabled, disabled)
		})
	}
}

func TestProviderLogoutAllowedForUnknownConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	// No calls are expected: there is no provider to log out of.
	githubProvider := mocks.NewMockAuthProvider(ctrl)
	SetProviders(map[string]common.AuthProvider{github.ProviderName: githubProvider})
	t.Cleanup(func() { SetProviders(nil) })
	SetAuthConfigLister(&fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
		},
	})
	t.Cleanup(func() { SetAuthConfigLister(nil) })

	// The deprecated auth provider names a registered provider, but it is
	// ignored: the principal's auth config doesn't exist.
	token := &apiv3.Token{
		AuthProvider:  github.ProviderName,
		UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}},
	}

	assert.NoError(t, ProviderLogout(nil, nil, token), "logout must not be blocked by a deleted auth config")
	assert.NoError(t, ProviderLogoutAll(nil, nil, token), "logout-all must not be blocked by a deleted auth config")
}

// fakeKnownPrincipalProvider is a local provider mock that can also look up
// known principals.
type fakeKnownPrincipalProvider struct {
	*mocks.MockAuthProvider
	known map[string]apiv3.Principal
	calls []string
}

func (p *fakeKnownPrincipalProvider) GetKnownPrincipal(principalID string, _ accessor.TokenAccessor) (apiv3.Principal, error) {
	p.calls = append(p.calls, principalID)
	principal, ok := p.known[principalID]
	if !ok {
		return apiv3.Principal{}, httperror.NewAPIError(httperror.NotFound, "not found")
	}
	return principal, nil
}

// TestGetPrincipalOfOtherConfig verifies that principals of other AuthConfigs
// are refused by the token's provider, unless MultiAuthProviderCross is
// enabled. Then they are looked up in Rancher's own users by the local
// provider, and never by the provider of their AuthConfig.
func TestGetPrincipalOfOtherConfig(t *testing.T) {
	t.Setenv("RANCHER_VERSION_TYPE", "prime")
	previous := features.MultiAuthProviderCross.Enabled()
	t.Cleanup(func() { features.MultiAuthProviderCross.Set(previous) })

	SetAuthConfigLister(&fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
			switch name {
			case "okta", "okta-eu":
				return &apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.OKTAConfigType}, nil
			case "openldap":
				return &apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: client.OpenLdapConfigType}, nil
			}
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
		},
	})
	t.Cleanup(func() { SetAuthConfigLister(nil) })

	oktaToken := &apiv3.Token{UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "okta_user://alice"}}}
	bob := apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "okta-eu_user://bob"}, DisplayName: "Bob"}
	carol := apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "openldap_user://carol"}, DisplayName: "Carol"}
	errNotIssued := httperror.NewAPIError(httperror.NotFound, "not issued by okta")

	// setup registers an okta provider and a local provider. The gomock
	// expectations set by each test are the only calls allowed.
	setup := func(t *testing.T) (*mocks.MockAuthProvider, *fakeKnownPrincipalProvider) {
		ctrl := gomock.NewController(t)
		okta := mocks.NewMockAuthProvider(ctrl)
		lp := &fakeKnownPrincipalProvider{
			MockAuthProvider: mocks.NewMockAuthProvider(ctrl),
			known:            map[string]apiv3.Principal{bob.Name: bob, carol.Name: carol},
		}
		SetProviders(map[string]common.AuthProvider{"okta": okta, local.Name: lp})
		t.Cleanup(func() { SetProviders(nil) })
		return okta, lp
	}

	t.Run("disabled: a principal of another config is refused by the token's provider", func(t *testing.T) {
		features.MultiAuthProviderCross.Set(false)
		okta, lp := setup(t)
		okta.EXPECT().GetPrincipal(bob.Name, oktaToken).Return(apiv3.Principal{}, errNotIssued)

		// The local provider mock expects no GetPrincipal call: the principal
		// isn't local, so there is no fallback to it.
		_, err := GetPrincipal(bob.Name, oktaToken)
		assert.ErrorIs(t, err, errNotIssued)
		assert.Empty(t, lp.calls)
	})

	t.Run("enabled: a principal of another config of the same type is a known user", func(t *testing.T) {
		features.MultiAuthProviderCross.Set(true)
		_, lp := setup(t)

		// The okta provider mock expects no calls: okta-eu's identity
		// provider is never queried.
		got, err := GetPrincipal(bob.Name, oktaToken)
		require.NoError(t, err)
		assert.Equal(t, bob, got)
		assert.Equal(t, []string{bob.Name}, lp.calls)
	})

	t.Run("enabled: a principal of a config of another type is a known user", func(t *testing.T) {
		features.MultiAuthProviderCross.Set(true)
		_, lp := setup(t)

		got, err := GetPrincipal(carol.Name, oktaToken)
		require.NoError(t, err)
		assert.Equal(t, carol, got)
		assert.Equal(t, []string{carol.Name}, lp.calls)
	})

	t.Run("enabled: an unknown principal of another config isn't found", func(t *testing.T) {
		features.MultiAuthProviderCross.Set(true)
		setup(t)

		_, err := GetPrincipal("okta-eu_user://dave", oktaToken)
		assert.True(t, httperror.IsNotFound(err), "expected a not found error, got %v", err)
	})

	t.Run("enabled: a principal of the token's config is looked up by its provider", func(t *testing.T) {
		features.MultiAuthProviderCross.Set(true)
		okta, lp := setup(t)
		alice := apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "okta_user://alice"}}
		okta.EXPECT().GetPrincipal(alice.Name, oktaToken).Return(alice, nil)

		got, err := GetPrincipal(alice.Name, oktaToken)
		require.NoError(t, err)
		assert.Equal(t, alice, got)
		assert.Empty(t, lp.calls)
	})

	t.Run("a local principal falls back to the local provider", func(t *testing.T) {
		features.MultiAuthProviderCross.Set(true)
		okta, lp := setup(t)
		localUser := apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "local://u-erin"}}
		okta.EXPECT().GetPrincipal(localUser.Name, oktaToken).Return(apiv3.Principal{}, errNotIssued)
		lp.EXPECT().GetPrincipal(localUser.Name, oktaToken).Return(localUser, nil)

		got, err := GetPrincipal(localUser.Name, oktaToken)
		require.NoError(t, err)
		assert.Equal(t, localUser, got)
		assert.Empty(t, lp.calls)
	})
}
