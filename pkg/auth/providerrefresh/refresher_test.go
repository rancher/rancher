package providerrefresh

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func Test_refreshAttributes(t *testing.T) {
	tests := []struct {
		name                  string
		user                  *v3.User
		attribs               *v3.UserAttribute
		providerDisabled      bool
		providerDisabledError error
		tokens                []*v3.Token
		enabled               bool
		deleted               bool
		want                  *v3.UserAttribute
	}{
		{
			name: "local user no tokens",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens:  []*v3.Token{},
			enabled: true,
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{},
			},
		},
		{
			name: "local user with login token",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    false,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
			},
			enabled: true,
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{
					providers.LocalProvider: {
						common.UserAttributePrincipalID: {"local://user-abcde"},
						common.UserAttributeUserName:    {"admin"},
					},
				},
			},
		},
		{
			name: "local user with derived token",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    true,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
			},
			enabled: true,
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{
					providers.LocalProvider: {
						common.UserAttributePrincipalID: {"local://user-abcde"},
						common.UserAttributeUserName:    {"admin"},
					},
				},
			},
		},
		{
			name: "user with derived token disabled in provider",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    true,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
			},
			enabled: false,
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{},
			},
		},
		{
			name: "user with login and derived tokens",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    false,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
				{
					UserID:       "user-abcde",
					IsDerived:    true,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-vwxyz",
							common.UserAttributeUserName:    "nimda",
						},
					},
				},
			},
			enabled: true,
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{
					providers.LocalProvider: {
						common.UserAttributePrincipalID: {"local://user-abcde"},
						common.UserAttributeUserName:    {"admin"},
					},
				},
			},
		},
		{
			name: "shibboleth user",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"shibboleth_user://user1",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    true,
					AuthProvider: saml.ShibbolethName,
					UserPrincipal: v3.Principal{
						Provider: saml.ShibbolethName,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "shibboleth_user://user1",
							common.UserAttributeUserName:    "user1",
						},
					},
				},
			},
			enabled: true,
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{
					saml.ShibbolethName: {
						common.UserAttributePrincipalID: {"shibboleth_user://user1"},
						common.UserAttributeUserName:    {"user1"},
					},
				},
			},
		},
		{
			name: "disabled provider, disabled/deleted tokens",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    false,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
				{
					UserID:       "user-abcde",
					IsDerived:    true,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
			},
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{},
			},
			providerDisabled: true,
			deleted:          true,
			enabled:          false,
		},
		{
			name: "error in determining if provider is disabled, tokens left unchanged",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				Username:   "admin",
				PrincipalIDs: []string{
					"local://user-abcde",
				},
			},
			attribs: &v3.UserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
			tokens: []*v3.Token{
				{
					UserID:       "user-abcde",
					IsDerived:    false,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
				{
					UserID:       "user-abcde",
					IsDerived:    true,
					AuthProvider: providers.LocalProvider,
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "local://user-abcde",
							common.UserAttributeUserName:    "admin",
						},
					},
				},
			},
			want: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
				GroupPrincipals: map[string]v3.Principals{
					"local":      {},
					"shibboleth": {},
				},
				ExtraByProvider: map[string]map[string][]string{
					providers.LocalProvider: {
						common.UserAttributePrincipalID: {"local://user-abcde"},
						common.UserAttributeUserName:    {"admin"},
					},
				},
			},
			providerDisabled:      true,
			providerDisabledError: fmt.Errorf("unable to determine if provider was disabled"),
			deleted:               false,
			enabled:               true,
		},
	}

	providers.ProviderNames = map[string]bool{
		providers.LocalProvider: true,
		saml.ShibbolethName:     true,
	}
	for _, tt := range tests {
		tokenUpdateCalled := false
		tokenDeleteCalled := false
		t.Run(tt.name, func(t *testing.T) {
			providers.Providers = map[string]common.AuthProvider{
				providers.LocalProvider: &mockLocalProvider{
					canAccess:   tt.enabled,
					disabled:    tt.providerDisabled,
					disabledErr: tt.providerDisabledError,
				},
				saml.ShibbolethName: &mockShibbolethProvider{},
			}
			r := &refresher{
				tokenLister: &fakes.TokenListerMock{
					ListFunc: func(_ string, _ labels.Selector) ([]*v3.Token, error) {
						return tt.tokens, nil
					},
				},
				userLister: &fakes.UserListerMock{
					GetFunc: func(_, _ string) (*v3.User, error) {
						return tt.user, nil
					},
				},
				tokens: &fakes.TokenInterfaceMock{
					DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
						tokenDeleteCalled = true
						return nil
					},
				},
				tokenMGR: tokens.NewMockedManager(&fakes.TokenInterfaceMock{
					UpdateFunc: func(_ *v3.Token) (*v3.Token, error) {
						tokenUpdateCalled = true
						return nil, nil
					},
				}),
			}
			got, err := r.refreshAttributes(tt.attribs)
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
			assert.NotEqual(t, tt.enabled, tokenUpdateCalled)
			assert.Equal(t, tt.deleted, tokenDeleteCalled)
		})
	}
}

func TestGetPrincipalIDForProvider(t *testing.T) {
	const testUserUsername = "tUser"
	tests := []struct {
		name               string
		userPrincipalIds   []string
		providerName       string
		desiredPrincipalId string
	}{
		{
			name:               "basic test",
			userPrincipalIds:   []string{fmt.Sprintf("azure_user://%s", testUserUsername)},
			providerName:       "azure",
			desiredPrincipalId: fmt.Sprintf("azure_user://%s", testUserUsername),
		},
		{
			name:               "no principal for provider",
			userPrincipalIds:   []string{fmt.Sprintf("azure_user://%s", testUserUsername)},
			providerName:       "not-a-provider",
			desiredPrincipalId: "",
		},
		{
			name:               "local provider principal",
			userPrincipalIds:   []string{fmt.Sprintf("local://%s", testUserUsername)},
			providerName:       "local",
			desiredPrincipalId: fmt.Sprintf("local://%s", testUserUsername),
		},
		{
			name:               "local provider missing principal",
			userPrincipalIds:   []string{fmt.Sprintf("local_user://%s", testUserUsername)},
			providerName:       "local",
			desiredPrincipalId: "",
		},
		{
			name: "multiple providers, correct one (first) chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("azure_user://%s", testUserUsername),
			},
			providerName:       "ldap",
			desiredPrincipalId: fmt.Sprintf("ldap_user://%s", testUserUsername),
		},
		{
			name: "multiple providers, correct one (last) chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("azure_user://%s", testUserUsername),
			},
			providerName:       "azure",
			desiredPrincipalId: fmt.Sprintf("azure_user://%s", testUserUsername),
		},
		{
			name: "multiple correct providers, first one chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("ldap_user://%s", "tUser2"),
			},
			providerName:       "ldap",
			desiredPrincipalId: fmt.Sprintf("ldap_user://%s", testUserUsername),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			user := v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: testUserUsername,
				},
				PrincipalIDs: test.userPrincipalIds,
			}
			outputPrincipalID := GetPrincipalIDForProvider(test.providerName, &user)
			assert.Equal(t, test.desiredPrincipalId, outputPrincipalID, "got a different principal id than expected")
		})
	}
}

type mockLocalProvider struct {
	canAccess   bool
	disabled    bool
	disabledErr error
}

func (p *mockLocalProvider) IsDisabledProvider() (bool, error) {
	return p.disabled, p.disabledErr
}

func (p *mockLocalProvider) GetName() string {
	panic("not implemented")
}

func (p *mockLocalProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	return token.UserPrincipal, nil
}

func (p *mockLocalProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockLocalProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return []v3.Principal{}, nil
}

func (p *mockLocalProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	return p.canAccess, nil
}

func (p *mockLocalProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: {userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    {userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockLocalProvider) CleanupResources(*v3.AuthConfig) error {
	return nil
}

type mockShibbolethProvider struct {
	enabled    bool
	enabledErr error
}

func (p *mockShibbolethProvider) IsDisabledProvider() (bool, error) {
	return p.enabled, p.enabledErr
}

func (p *mockShibbolethProvider) GetName() string {
	panic("not implemented")
}

func (p *mockShibbolethProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	return token.UserPrincipal, nil
}

func (p *mockShibbolethProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return []v3.Principal{}, errors.New("Not implemented")
}

func (p *mockShibbolethProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	return true, nil
}

func (p *mockShibbolethProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: {userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    {userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockShibbolethProvider) CleanupResources(*v3.AuthConfig) error {
	return nil
}
