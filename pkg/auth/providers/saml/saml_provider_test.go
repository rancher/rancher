package saml

import (
	"context"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestConfiguredOktaProviderContainsLdapProvider(t *testing.T) {
	// saml.Configure runs some ldap specific logic based on the saml provider name, so we provide
	// just enough scaffolding to run the Configure function.
	ctx := context.Background()
	mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
	require.NoError(t, err, "Failed to create NewScaledContext")
	tokenMGR := tokens.NewManager(ctx, mgmtCtx)
	provider, ok := Configure(ctx, mgmtCtx, mgmtCtx.UserManager, tokenMGR, "okta").(*Provider)
	require.True(t, ok, "Failed to Configure a valid Provider")

	assert.True(t, provider.hasLdapGroupSearch(), "Missing LDAP group search capability for okta provider")
	assert.NotNil(t, provider.ldapProvider, "Configured okta provider did not receive child LDAP provider")
}

func TestSearchPrincipals(t *testing.T) {
	providerName := "okta"
	userType := "okta_user"
	groupType := "okta_group"

	tests := []struct {
		desc             string
		searchKey        string
		principalType    string
		isLdapConfigured bool
		principals       []string
	}{
		{
			desc:             "search for user with ldap",
			isLdapConfigured: true,
			searchKey:        "al",
			principalType:    common.UserPrincipalType,
			principals: []string{
				userType + "://alice",
			},
		},
		{
			desc:             "search for user without ldap",
			isLdapConfigured: false,
			searchKey:        "alice",
			principalType:    common.UserPrincipalType,
			principals: []string{
				userType + "://alice",
			},
		},
		{
			desc:             "search for group without ldap",
			isLdapConfigured: false,
			searchKey:        "admins",
			principalType:    common.GroupPrincipalType,
			principals: []string{
				groupType + "://admins",
			},
		},
		{
			desc:             "search for any principal without ldap",
			isLdapConfigured: false,
			searchKey:        "dev",
			principalType:    "",
			principals: []string{
				userType + "://dev",
				groupType + "://dev",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			provider := &Provider{
				name:      providerName,
				userType:  userType,
				groupType: groupType,
				ldapProvider: &mockLdapProvider{
					providerName:     providerName,
					isLdapConfigured: tt.isLdapConfigured,
				},
			}

			results, err := provider.SearchPrincipals(tt.searchKey, tt.principalType, v3.Token{})
			require.NoError(t, err)
			require.Len(t, results, len(tt.principals))
			for _, principal := range results {
				assert.Contains(t, tt.principals, principal.Name)
			}
		})
	}
}

// Bare minimum to provide ldap responses (or error conditions) when performing SearchPrincipals. We're testing
// the SAML provider's logic, not anything the ldap provider is doing, so we merely need enough scaffolding to
// detect that the ldapProvider was used at all.
type mockLdapProvider struct {
	providerName     string
	isLdapConfigured bool
}

func (p *mockLdapProvider) GetName() string {
	return p.providerName
}

func (p *mockLdapProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("AuthenticateUser Unimplemented!")
}

func (p *mockLdapProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	if !p.isLdapConfigured {
		return nil, ldap.ErrorNotConfigured{}
	}

	return []v3.Principal{{
		ObjectMeta:    metav1.ObjectMeta{Name: p.providerName + "_" + principalType + "://alice"},
		DisplayName:   "Alice",
		LoginName:     "alice",
		PrincipalType: "user",
		Me:            true,
		Provider:      p.providerName,
	}}, nil
}

func (p *mockLdapProvider) CustomizeSchema(schema *types.Schema) {
	panic("CustomizeSchema Unimplemented!")
}

func (p *mockLdapProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	panic("GetPrincipal Unimplemented!")
}

func (p *mockLdapProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("TransformToAuthProvider Unimplemented!")
}

func (p *mockLdapProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	panic("RefetchGroupPrincipals Unimplemented!")
}

func (p *mockLdapProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	panic("CanAccessWithGroupProviders Unimplemented!")
}

func (p *mockLdapProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	panic("GetUserExtraAttributes Unimplemented!")
}

func (p *mockLdapProvider) IsDisabledProvider() (bool, error) {
	panic("IsDisabledProvider Unimplemented!")
}
