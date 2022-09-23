package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/cleanup"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockAzureProvider struct {
	secrets v1.SecretInterface
}

func (p *mockAzureProvider) GetName() string {
	return "mockAzureAD"
}

func (p *mockAzureProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockAzureProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *mockAzureProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	return token.UserPrincipal, nil
}

func (p *mockAzureProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockAzureProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *mockAzureProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return []v3.Principal{}, nil
}

func (p *mockAzureProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	return true, nil
}

func (p *mockAzureProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: {userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    {userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockAzureProvider) IsDisabledProvider() (bool, error) {
	return true, nil
}

func (p *mockAzureProvider) CleanupResources(*v3.AuthConfig) error {
	return p.secrets.DeleteNamespaced(common.SecretsNamespace, clients.AccessTokenSecretName, &metav1.DeleteOptions{})
}

func getSecretInterfaceMock(store map[string]*corev1.Secret) v1.SecretInterface {
	secretInterfaceMock := &fakes.SecretInterfaceMock{}

	secretInterfaceMock.CreateFunc = func(secret *corev1.Secret) (*corev1.Secret, error) {
		if secret.Name == "" {
			uniqueIdentifier := md5.Sum([]byte(time.Now().String()))
			secret.Name = hex.EncodeToString(uniqueIdentifier[:])
		}
		store[fmt.Sprintf("%s:%s", secret.Namespace, secret.Name)] = secret
		return secret, nil
	}

	secretInterfaceMock.GetNamespacedFunc = func(namespace string, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
		secret, ok := store[fmt.Sprintf("%s:%s", namespace, name)]
		if ok {
			return secret, nil
		}
		return nil, errors.New("secret not found")
	}

	secretInterfaceMock.DeleteNamespacedFunc = func(namespace string, name string, options *metav1.DeleteOptions) error {
		delete(store, fmt.Sprintf("%s:%s", namespace, name))
		return nil
	}

	return secretInterfaceMock
}

func TestSyncTriggersCleanupOnDisable(t *testing.T) {
	mockSecrets := make(map[string]*corev1.Secret)
	provider := &mockAzureProvider{
		secrets: getSecretInterfaceMock(mockSecrets),
	}

	providers.Providers = map[string]common.AuthProvider{
		"mockAzureAD": provider,
	}

	_, err := provider.secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clients.AccessTokenSecretName,
			Namespace: common.SecretsNamespace,
		},
		StringData: map[string]string{"access-token": "my JWT token"},
	})
	assert.NoError(t, err)

	s, err := provider.secrets.GetNamespaced(common.SecretsNamespace, clients.AccessTokenSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, s.Name, clients.AccessTokenSecretName)

	mockRefresher := newMockAuthProvider()
	mockUsers := newMockUserLister()
	mockCleanupService := cleanup.NewCleanupService()

	controller := authConfigController{users: &mockUsers, authRefresher: &mockRefresher, cleanup: mockCleanupService}
	config := &v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "mockAzureAD"},
		Enabled:    false,
	}

	_, err = controller.sync("test", config)
	assert.NoError(t, err)

	s, err = provider.secrets.GetNamespaced(common.SecretsNamespace, clients.AccessTokenSecretName, metav1.GetOptions{})
	assert.Error(t, err, "expected not to find the secret belonging to the disabled auth provider")
	assert.Nil(t, s, "expected the secret to be nil")
}

func TestSyncDoesNotTriggerCleanup(t *testing.T) {
	mockSecrets := make(map[string]*corev1.Secret)
	provider := &mockAzureProvider{
		secrets: getSecretInterfaceMock(mockSecrets),
	}

	providers.Providers = map[string]common.AuthProvider{
		"mockAzureAD": provider,
	}

	_, err := provider.secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clients.AccessTokenSecretName,
			Namespace: common.SecretsNamespace,
		},
		StringData: map[string]string{"access-token": "my JWT token"},
	})
	assert.NoError(t, err)

	s, err := provider.secrets.GetNamespaced(common.SecretsNamespace, clients.AccessTokenSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, s.Name, clients.AccessTokenSecretName)

	mockRefresher := newMockAuthProvider()
	mockUsers := newMockUserLister()
	mockCleanupService := cleanup.NewCleanupService()

	controller := authConfigController{users: &mockUsers, authRefresher: &mockRefresher, cleanup: mockCleanupService}
	config := &v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "mockAzureAD"},
		Enabled:    true,
	}

	// Since the config is enabled, the cleanup routine must not be triggered.
	_, err = controller.sync("test", config)
	assert.NoError(t, err)

	s, err = provider.secrets.GetNamespaced(common.SecretsNamespace, clients.AccessTokenSecretName, metav1.GetOptions{})
	assert.NoError(t, err, "expected to find the secret belonging to the disabled auth provider")
	assert.NotNil(t, s, "expected the secret to not be nil")
}

type mockUserLister struct {
	users        []*v3.User
	listUsersErr error
}

func newMockUserLister() mockUserLister {
	return mockUserLister{
		users: []*v3.User{},
	}
}

func (m *mockUserLister) List(namespace string, selector labels.Selector) (ret []*v3.User, err error) {
	if m.listUsersErr != nil {
		return nil, m.listUsersErr
	}
	return m.users, nil
}
func (m *mockUserLister) Get(namespace, name string) (*v3.User, error) {
	for _, user := range m.users {
		if user.Name == name {
			return user, nil
		}
	}
	return nil, apierror.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "user"}, name)
}

func (m *mockUserLister) AddUser(username string, provider string) {
	principalIds := []string{
		fmt.Sprintf("local://%s", username),
		fmt.Sprintf("%s_user://%s", provider, username),
	}
	newUser := v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: username},
		PrincipalIDs: principalIds,
	}
	found := false
	for idx, user := range m.users {
		if user.Name == newUser.Name {
			m.users[idx] = &newUser
			found = true
		}
	}
	if !found {
		m.users = append(m.users, &newUser)
	}
}

func (m *mockUserLister) AddListUserError(err error) {
	m.listUsersErr = err
}

type mockAuthProvider struct {
	allUsersRefreshed bool
	refreshedUsers    map[string]bool
}

func newMockAuthProvider() mockAuthProvider {
	return mockAuthProvider{
		allUsersRefreshed: false,
		refreshedUsers:    map[string]bool{},
	}
}

func (m *mockAuthProvider) TriggerAllUserRefresh() {
	m.allUsersRefreshed = true
}

func (m *mockAuthProvider) TriggerUserRefresh(username string, force bool) {
	m.refreshedUsers[username] = force
}
