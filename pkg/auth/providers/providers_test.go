package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewAzureADProviderDoesNotHavePerUserTokens(t *testing.T) {
	t.Cleanup(cleanup)
	newFlowCfg := map[string]any{
		"metadata": map[string]any{
			"name":        "azure",
			"annotations": map[string]any{"auth.cattle.io/azuread-endpoint-migrated": "true"},
		},
		"enabled":       true,
		"graphEndpoint": "https://graph.microsoft.com/",
	}

	getter := newMockUnstructuredGetter()
	obj := mockUnstructured{content: newFlowCfg}
	getter.objects[azure.Name] = &obj
	Providers[azure.Name] = &azure.Provider{Retriever: getter}

	hasPerUserSecrets, err := ProviderHasPerUserSecrets(azure.Name)

	require.NoError(t, err)
	assert.False(t, hasPerUserSecrets)
}

func TestOldAzureADProviderHasPerUserTokens(t *testing.T) {
	t.Cleanup(cleanup)
	oldFlowCfg := map[string]any{
		"metadata": map[string]any{
			"name": "azure",
		},
		"enabled":       true,
		"graphEndpoint": "https://graph.windows.net/",
	}

	getter := newMockUnstructuredGetter()
	obj := mockUnstructured{content: oldFlowCfg}
	getter.objects[azure.Name] = &obj
	Providers[azure.Name] = &azure.Provider{Retriever: getter}

	hasPerUserSecrets, err := ProviderHasPerUserSecrets(azure.Name)

	require.NoError(t, err)
	assert.True(t, hasPerUserSecrets)
}

func TestBadAzureProviderDoesNotHavePerUserTokens(t *testing.T) {
	t.Run("Azure Provider is not registered", func(t *testing.T) {
		t.Cleanup(cleanup)
		hasPerUserSecrets, err := ProviderHasPerUserSecrets(azure.Name)

		require.Error(t, err)
		assert.False(t, hasPerUserSecrets)
	})

	t.Run("Azure Provider has the wrong type", func(t *testing.T) {
		t.Cleanup(cleanup)
		Providers[azure.Name] = fakeProvider{}
		hasPerUserSecrets, err := ProviderHasPerUserSecrets(azure.Name)

		require.Error(t, err)
		assert.False(t, hasPerUserSecrets)
	})

	t.Run("Config could not be fetch from Kubernetes", func(t *testing.T) {
		t.Cleanup(cleanup)
		getter := newMockUnstructuredGetter()
		getter.errObjects[azure.Name] = errors.New("error getting config")
		Providers[azure.Name] = &azure.Provider{Retriever: getter}
		hasPerUserSecrets, err := ProviderHasPerUserSecrets(azure.Name)

		require.Error(t, err)
		assert.False(t, hasPerUserSecrets)
	})
}

func TestProviderHasPerUserTokens(t *testing.T) {
	t.Cleanup(cleanup)
	hasPerUserSecrets, err := ProviderHasPerUserSecrets(github.Name)

	require.NoError(t, err)
	assert.False(t, hasPerUserSecrets)

	providersWithSecrets[github.Name] = true
	hasPerUserSecrets, err = ProviderHasPerUserSecrets(github.Name)

	require.NoError(t, err)
	assert.True(t, hasPerUserSecrets)
}

func TestGetEnabledExternalProvider(t *testing.T) {
	t.Cleanup(cleanup)

	Providers[local.Name] = fakeProvider{
		name:     local.Name,
		disabled: false,
	}
	Providers[ldap.OpenLdapName] = fakeProvider{
		name:     ldap.OpenLdapName,
		disabled: true,
	}
	Providers[activedirectory.Name] = fakeProvider{
		name:     activedirectory.Name,
		disabled: false,
	}
	Providers[github.Name] = fakeProvider{
		name:     github.Name,
		disabled: false,
	}

	t.Run("Enabled external provider is returned", func(t *testing.T) {
		provider := GetEnabledExternalProvider()
		require.NotNil(t, provider)
		// Either one of the enabled ones is returned, but not the local.
		assert.Contains(t, []string{activedirectory.Name, github.Name}, provider.GetName())
	})

	delete(Providers, github.Name)
	delete(Providers, activedirectory.Name)

	t.Run("No enabled external provider", func(t *testing.T) {
		provider := GetEnabledExternalProvider()
		require.Nil(t, provider)
	})
}

func TestGetPrincipal(t *testing.T) {
	localPrincipalID := "local://user2"
	extPrincipalID := "activedirectory_user://CN=user2,OU=CN\\=test,DC=test-ad,DC=ad,DC=com"

	localToken := v3.Token{AuthProvider: local.Name, UserID: "local://user1"}
	extToken := v3.Token{AuthProvider: activedirectory.Name, UserID: "activedirectory_user://CN=user1,OU=CN\\=test,DC=test-ad,DC=ad,DC=com"}

	tests := []struct {
		desc              string
		myToken           v3.Token
		localProvider     common.AuthProvider
		extProvider       common.AuthProvider
		searchPrincipalID string
		wantProvider      string
		shoudErr          bool
	}{
		{
			desc:    "Local user resolving local principal",
			myToken: localToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					return v3.Principal{
						ObjectMeta:    metav1.ObjectMeta{Name: localPrincipalID},
						Provider:      local.Name,
						PrincipalType: "user",
					}, nil
				},
			},
			searchPrincipalID: localPrincipalID,
			wantProvider:      local.Name,
		},
		{
			desc:    "Local user resolving external principal",
			myToken: localToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					t.Error("Unexpected call to local provider")
					return v3.Principal{}, nil
				},
			},
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					return v3.Principal{
						ObjectMeta:    metav1.ObjectMeta{Name: extPrincipalID},
						Provider:      activedirectory.Name,
						PrincipalType: "user",
					}, nil
				},
			},
			searchPrincipalID: extPrincipalID,
			wantProvider:      activedirectory.Name,
		},
		{
			desc:    "External user resolving external principal",
			myToken: extToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					t.Error("Unexpected call to local provider")
					return v3.Principal{}, nil
				},
			},
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					return v3.Principal{
						ObjectMeta:    metav1.ObjectMeta{Name: extPrincipalID},
						Provider:      activedirectory.Name,
						PrincipalType: "user",
					}, nil
				},
			},
			searchPrincipalID: extPrincipalID,
			wantProvider:      activedirectory.Name,
		},
		{
			desc:    "External user resolving local principal",
			myToken: extToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					return v3.Principal{
						ObjectMeta:    metav1.ObjectMeta{Name: localPrincipalID},
						Provider:      local.Name,
						PrincipalType: "user",
					}, nil
				},
			},
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: false,
				getPrincipalFunc: func(string, v3.Token) (v3.Principal, error) {
					t.Error("Unexpected call to external provider")
					return v3.Principal{}, nil
				},
			},
			searchPrincipalID: localPrincipalID,
			wantProvider:      local.Name,
		},
		{
			desc:              "Provider is not initialized",
			myToken:           localToken,
			searchPrincipalID: extPrincipalID,
			shoudErr:          true,
		},
		{
			desc:    "Provider disabled",
			myToken: localToken,
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: true,
			},
			searchPrincipalID: extPrincipalID,
			shoudErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Cleanup(cleanup)

			if tt.localProvider != nil {
				Providers[local.Name] = tt.localProvider
			}
			if tt.extProvider != nil {
				Providers[activedirectory.Name] = tt.extProvider
			}

			principal, err := GetPrincipal(tt.searchPrincipalID, tt.myToken)
			if tt.shoudErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.searchPrincipalID, principal.Name)
			assert.Equal(t, tt.wantProvider, principal.Provider)
		})
	}
}

func TestSearchPrincipals(t *testing.T) {
	originalLocalSearch := searchLocalPrincipalsDedupe
	t.Cleanup(func() {
		searchLocalPrincipalsDedupe = originalLocalSearch
		cleanup()
	})

	localPrincipalID := "local://user2"
	extPrincipalID := "activedirectory_user://CN=user2,OU=CN\\=test,DC=test-ad,DC=ad,DC=com"

	localToken := v3.Token{AuthProvider: local.Name, UserID: "local://user1"}
	extToken := v3.Token{AuthProvider: activedirectory.Name, UserID: "activedirectory_user://CN=user1,OU=CN\\=test,DC=test-ad,DC=ad,DC=com"}

	tests := []struct {
		desc           string
		myToken        v3.Token
		localProvider  common.AuthProvider
		extProvider    common.AuthProvider
		localSearch    dedupeSearchFunc
		wantPrincipals []string
	}{
		{
			desc:    "Search as a local user",
			myToken: localToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
			},
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: false,
				searchPrincipalsFunc: func(string, string, v3.Token) ([]v3.Principal, error) {
					return []v3.Principal{{
						ObjectMeta:    metav1.ObjectMeta{Name: extPrincipalID},
						Provider:      activedirectory.Name,
						PrincipalType: "user",
					}}, nil
				},
			},
			localSearch: func(string, string, v3.Token, []v3.Principal) ([]v3.Principal, error) {
				return []v3.Principal{{
					ObjectMeta:    metav1.ObjectMeta{Name: localPrincipalID},
					Provider:      local.Name,
					PrincipalType: "user",
				}}, nil
			},
			wantPrincipals: []string{localPrincipalID, extPrincipalID},
		},
		{
			desc:    "Search as an external user",
			myToken: extToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
			},
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: false,
				searchPrincipalsFunc: func(string, string, v3.Token) ([]v3.Principal, error) {
					return []v3.Principal{{
						ObjectMeta:    metav1.ObjectMeta{Name: extPrincipalID},
						Provider:      activedirectory.Name,
						PrincipalType: "user",
					}}, nil
				},
			},
			localSearch: func(string, string, v3.Token, []v3.Principal) ([]v3.Principal, error) {
				return []v3.Principal{{
					ObjectMeta:    metav1.ObjectMeta{Name: localPrincipalID},
					Provider:      local.Name,
					PrincipalType: "user",
				}}, nil
			},
			wantPrincipals: []string{localPrincipalID, extPrincipalID},
		},
		{
			desc:    "Search only local provider",
			myToken: extToken,
			localProvider: &fakeProvider{
				name:     local.Name,
				disabled: false,
			},
			extProvider: &fakeProvider{
				name:     activedirectory.Name,
				disabled: true,
			},
			localSearch: func(string, string, v3.Token, []v3.Principal) ([]v3.Principal, error) {
				return []v3.Principal{{
					ObjectMeta:    metav1.ObjectMeta{Name: localPrincipalID},
					Provider:      local.Name,
					PrincipalType: "user",
				}}, nil
			},
			wantPrincipals: []string{localPrincipalID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.localProvider != nil {
				Providers[local.Name] = tt.localProvider
			}
			if tt.extProvider != nil {
				Providers[activedirectory.Name] = tt.extProvider
			}

			searchLocalPrincipalsDedupe = tt.localSearch

			found, err := SearchPrincipals("user", "", tt.myToken)
			require.NoError(t, err)
			assert.Len(t, found, len(tt.wantPrincipals))
			for _, principal := range found {
				assert.Contains(t, tt.wantPrincipals, principal.Name)
			}
		})
	}
}

func cleanup() {
	Providers = make(map[string]common.AuthProvider)
	providersWithSecrets = make(map[string]bool)
}

type mockUnstructuredGetter struct {
	objects    map[string]runtime.Object
	errObjects map[string]error
}

func newMockUnstructuredGetter() *mockUnstructuredGetter {
	return &mockUnstructuredGetter{
		objects:    map[string]runtime.Object{},
		errObjects: map[string]error{},
	}
}

func (m *mockUnstructuredGetter) Get(name string, _ metav1.GetOptions) (runtime.Object, error) {
	if obj, ok := m.objects[name]; ok {
		return obj, nil
	}
	if err, ok := m.errObjects[name]; ok {
		return nil, err
	}
	return nil, fmt.Errorf("object %s not found", name)
}

type mockUnstructured struct {
	content map[string]interface{}
}

func (m *mockUnstructured) NewEmptyInstance() runtime.Unstructured                 { return nil }
func (m *mockUnstructured) UnstructuredContent() map[string]interface{}            { return m.content }
func (m *mockUnstructured) SetUnstructuredContent(input map[string]interface{})    { m.content = input }
func (m *mockUnstructured) IsList() bool                                           { return false }
func (m *mockUnstructured) EachListItem(func(runtime.Object) error) error          { return nil }
func (m *mockUnstructured) EachListItemWithAlloc(func(runtime.Object) error) error { return nil }
func (m *mockUnstructured) GetObjectKind() schema.ObjectKind                       { return nil }
func (m *mockUnstructured) DeepCopyObject() runtime.Object                         { return nil }

type fakeProvider struct {
	name                 string
	disabled             bool
	searchPrincipalsFunc func(string, string, v3.Token) ([]v3.Principal, error)
	getPrincipalFunc     func(string, v3.Token) (v3.Principal, error)
}

func (f fakeProvider) GetName() string {
	return f.name
}

func (f fakeProvider) AuthenticateUser(context.Context, interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("implement me")
}

func (f fakeProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	if f.searchPrincipalsFunc != nil {
		return f.searchPrincipalsFunc(name, principalType, myToken)
	}
	return nil, nil
}

func (f fakeProvider) GetPrincipal(principalID string, myToken v3.Token) (v3.Principal, error) {
	if f.getPrincipalFunc != nil {
		return f.getPrincipalFunc(principalID, myToken)
	}
	return v3.Principal{}, nil
}

func (f fakeProvider) CustomizeSchema(*types.Schema) {
	panic("implement me")
}

func (f fakeProvider) TransformToAuthProvider(map[string]interface{}) (map[string]interface{}, error) {
	panic("implement me")
}

func (f fakeProvider) RefetchGroupPrincipals(string, string) ([]v3.Principal, error) {
	panic("implement me")
}

func (f fakeProvider) CanAccessWithGroupProviders(string, []v3.Principal) (bool, error) {
	panic("implement me")
}

func (f fakeProvider) GetUserExtraAttributes(v3.Principal) map[string][]string {
	panic("implement me")
}

func (f fakeProvider) IsDisabledProvider() (bool, error) {
	return f.disabled, nil
}
