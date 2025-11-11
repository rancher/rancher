package providers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsSAMLProvider(t *testing.T) {
	tests := []struct {
		provider string
		isSAML   bool
	}{
		{
			provider: "pingProvider",
			isSAML:   true,
		},
		{
			provider: "adfsProvider",
			isSAML:   true,
		},
		{
			provider: "keyCloakProvider",
			isSAML:   true,
		},
		{
			provider: "oktaProvider",
			isSAML:   true,
		},
		{
			provider: "shibbolethProvider",
			isSAML:   true,
		},
		{
			provider: "githubProvider",
			isSAML:   false,
		},
		{
			provider: "localProvider",
			isSAML:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			assert.Equal(t, tt.isSAML, IsSAMLProviderType(tt.provider))
		})
	}
}

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
	content map[string]any
}

func (m *mockUnstructured) NewEmptyInstance() runtime.Unstructured                 { return nil }
func (m *mockUnstructured) UnstructuredContent() map[string]any                    { return m.content }
func (m *mockUnstructured) SetUnstructuredContent(input map[string]any)            { m.content = input }
func (m *mockUnstructured) IsList() bool                                           { return false }
func (m *mockUnstructured) EachListItem(func(runtime.Object) error) error          { return nil }
func (m *mockUnstructured) EachListItemWithAlloc(func(runtime.Object) error) error { return nil }
func (m *mockUnstructured) GetObjectKind() schema.ObjectKind                       { return nil }
func (m *mockUnstructured) DeepCopyObject() runtime.Object                         { return nil }

type fakeProvider struct{}

func (f fakeProvider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (f fakeProvider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (f fakeProvider) GetName() string {
	panic("implement me")
}

func (f fakeProvider) AuthenticateUser(http.ResponseWriter, *http.Request, any) (v3.Principal, []v3.Principal, string, error) {
	panic("implement me")
}

func (f fakeProvider) SearchPrincipals(_, _ string, _ accessor.TokenAccessor) ([]v3.Principal, error) {
	panic("implement me")
}

func (f fakeProvider) GetPrincipal(_ string, _ accessor.TokenAccessor) (v3.Principal, error) {
	panic("implement me")
}

func (f fakeProvider) CustomizeSchema(_ *types.Schema) {
	panic("implement me")
}

func (f fakeProvider) TransformToAuthProvider(_ map[string]any) (map[string]any, error) {
	panic("implement me")
}

func (f fakeProvider) RefetchGroupPrincipals(_ string, _ string) ([]v3.Principal, error) {
	panic("implement me")
}

func (f fakeProvider) CanAccessWithGroupProviders(_ string, _ []v3.Principal) (bool, error) {
	panic("implement me")
}

func (f fakeProvider) GetUserExtraAttributes(_ v3.Principal) map[string][]string {
	panic("implement me")
}

func (f fakeProvider) GetUserExtraAttributesFromToken(_ accessor.TokenAccessor) map[string][]string {
	panic("implement me")
}

func (f fakeProvider) IsDisabledProvider() (bool, error) {
	panic("implement me")
}
