package cleanup

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func TestRunCleanup(t *testing.T) {
	var globalRoleBindingStore = map[string]*v3.GlobalRoleBinding{
		"azure": {
			ObjectMeta:         metav1.ObjectMeta{Name: "azure"},
			GroupPrincipalName: "azuread_group://mygroup",
		},
		"ping": {
			ObjectMeta:         metav1.ObjectMeta{Name: "ping"},
			GroupPrincipalName: "ping_group://mygroup",
		},
	}

	var projectRoleTemplateBindingStore = map[string]*v3.ProjectRoleTemplateBinding{
		"local:azure": {
			ObjectMeta:         metav1.ObjectMeta{Name: "azure", Namespace: "local"},
			GroupPrincipalName: "azuread_group://mygroup",
		},

		"local:ping": {
			ObjectMeta:         metav1.ObjectMeta{Name: "ping", Namespace: "local"},
			GroupPrincipalName: "ping_group://mygroup",
		},
	}

	var clusterRoleTemplateBindingStore = map[string]*v3.ClusterRoleTemplateBinding{
		"local:azure": {
			ObjectMeta:         metav1.ObjectMeta{Name: "ping", Namespace: "local"},
			GroupPrincipalName: "azuread_group://mygroup",
		},
		"local:ping": {
			ObjectMeta:         metav1.ObjectMeta{Name: "ping", Namespace: "local"},
			GroupPrincipalName: "ping_group://mygroup",
		},
	}

	var userStore = map[string]*v3.User{
		"alice": {
			ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
			PrincipalIDs: []string{"azuread_group://alice"},
		},
		"bob": {
			ObjectMeta:   metav1.ObjectMeta{Name: "bob"},
			PrincipalIDs: []string{"local://bob"},
		},
		"rick": {
			ObjectMeta:   metav1.ObjectMeta{Name: "rick"},
			PrincipalIDs: []string{"azuread_group://rick", "local://rick"},
			Password:     "secret",
		},
	}

	var secretStore = map[string]*v1.Secret{
		"cattle-system:oauthSecretName": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oauthSecretName",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"azuread": []byte("my user token"),
			},
		},
		"cattle-global-data:azureadconfig-applicationsecret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", strings.ToLower(client.AzureADConfigType), client.AzureADConfigFieldApplicationSecret),
				Namespace: common.SecretsNamespace,
			},
		},
		"cattle-global-data:azuread-access-token": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", strings.ToLower("azuread"), "access-token"),
				Namespace: common.SecretsNamespace,
			},
		},
		"foo:regular-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "regular secret",
				Namespace: "foo",
			},
		},
	}

	svc := newMockCleanupService(
		globalRoleBindingStore,
		projectRoleTemplateBindingStore,
		clusterRoleTemplateBindingStore,
		userStore,
		secretStore,
	)
	cfg := v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "azuread",
		},
		Type:    client.AzureADConfigType,
		Enabled: false,
	}

	err := svc.Run(&cfg)

	require.NoError(t, err)
	assert.Len(t, globalRoleBindingStore, 1)
	assert.Len(t, clusterRoleTemplateBindingStore, 1)
	assert.Len(t, projectRoleTemplateBindingStore, 1)
	assert.Len(t, userStore, 2)
	assert.Len(t, secretStore, 1)

	for _, user := range userStore {
		require.Lenf(t, user.PrincipalIDs, 1, "every user after cleanup must have only one principal ID, got %d", len(user.PrincipalIDs))
		principalID := user.PrincipalIDs[0]
		assert.Truef(t, strings.HasPrefix(principalID, "local"), "the only principal ID has 'local' as a prefix, got %s", principalID)
	}
}

func newMockCleanupService(
	grbStore map[string]*v3.GlobalRoleBinding,
	prtbStore map[string]*v3.ProjectRoleTemplateBinding,
	crtbStore map[string]*v3.ClusterRoleTemplateBinding,
	userStore map[string]*v3.User,
	secretStore map[string]*v1.Secret) Service {
	return Service{
		secretsInterface:                  getSecretInterfaceMock(secretStore),
		globalRoleBindingsCache:           mockGlobalRoleBindingCache{grbStore},
		globalRoleBindingsClient:          mockGlobalRoleBindingClient{grbStore},
		projectRoleTemplateBindingsCache:  mockProjectRoleTemplateBindingCache{prtbStore},
		projectRoleTemplateBindingsClient: mockProjectRoleTemplateBindingClient{prtbStore},
		clusterRoleTemplateBindingsCache:  mockClusterRoleTemplateBindingCache{crtbStore},
		clusterRoleTemplateBindingsClient: mockClusterRoleTemplateBindingClient{crtbStore},
		userCache:                         mockUserCache{userStore},
		userClient:                        mockUserClient{userStore},
	}
}

type mockGlobalRoleBindingCache struct {
	store map[string]*v3.GlobalRoleBinding
}

func (m mockGlobalRoleBindingCache) Get(name string) (*v3.GlobalRoleBinding, error) {
	return m.store[name], nil
}

func (m mockGlobalRoleBindingCache) List(_ labels.Selector) ([]*v3.GlobalRoleBinding, error) {
	var lst []*v3.GlobalRoleBinding
	for _, v := range m.store {
		lst = append(lst, v)
	}
	return lst, nil
}

func (m mockGlobalRoleBindingCache) AddIndexer(_ string, _ controllers.GlobalRoleBindingIndexer) {
	panic("not implemented")
}

func (m mockGlobalRoleBindingCache) GetByIndex(_, _ string) ([]*v3.GlobalRoleBinding, error) {
	panic("not implemented")
}

type mockGlobalRoleBindingClient struct {
	store map[string]*v3.GlobalRoleBinding
}

func (m mockGlobalRoleBindingClient) Create(_ *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	panic("not implemented")
}

func (m mockGlobalRoleBindingClient) Update(_ *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	panic("not implemented")
}

func (m mockGlobalRoleBindingClient) Delete(name string, _ *metav1.DeleteOptions) error {
	delete(m.store, name)
	return nil
}

func (m mockGlobalRoleBindingClient) Get(_ string, _ metav1.GetOptions) (*v3.GlobalRoleBinding, error) {
	panic("not implemented")
}

func (m mockGlobalRoleBindingClient) List(_ metav1.ListOptions) (*v3.GlobalRoleBindingList, error) {
	panic("not implemented")
}

func (m mockGlobalRoleBindingClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}

func (m mockGlobalRoleBindingClient) Patch(_ string, _ apitypes.PatchType, _ []byte, _ ...string) (result *v3.GlobalRoleBinding, err error) {
	panic("not implemented")
}

type mockProjectRoleTemplateBindingCache struct {
	store map[string]*v3.ProjectRoleTemplateBinding
}

func (m mockProjectRoleTemplateBindingCache) Get(namespace, name string) (*v3.ProjectRoleTemplateBinding, error) {
	return m.store[namespace+":"+name], nil
}

func (m mockProjectRoleTemplateBindingCache) List(_ string, _ labels.Selector) ([]*v3.ProjectRoleTemplateBinding, error) {
	var lst []*v3.ProjectRoleTemplateBinding
	for _, v := range m.store {
		lst = append(lst, v)
	}
	return lst, nil
}

func (m mockProjectRoleTemplateBindingCache) AddIndexer(_ string, _ controllers.ProjectRoleTemplateBindingIndexer) {
	panic("not implemented")
}

func (m mockProjectRoleTemplateBindingCache) GetByIndex(_, _ string) ([]*v3.ProjectRoleTemplateBinding, error) {
	panic("not implemented")
}

type mockProjectRoleTemplateBindingClient struct {
	store map[string]*v3.ProjectRoleTemplateBinding
}

func (m mockProjectRoleTemplateBindingClient) Create(_ *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	panic("not implemented")
}

func (m mockProjectRoleTemplateBindingClient) Update(_ *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	panic("not implemented")
}

func (m mockProjectRoleTemplateBindingClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	delete(m.store, namespace+":"+name)
	return nil
}

func (m mockProjectRoleTemplateBindingClient) Get(namespace, name string, _ metav1.GetOptions) (*v3.ProjectRoleTemplateBinding, error) {
	return m.store[namespace+":"+name], nil
}

func (m mockProjectRoleTemplateBindingClient) List(_ string, _ metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error) {
	panic("not implemented")
}

func (m mockProjectRoleTemplateBindingClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}

func (m mockProjectRoleTemplateBindingClient) Patch(_, _ string, _ apitypes.PatchType, _ []byte, _ ...string) (result *v3.ProjectRoleTemplateBinding, err error) {
	panic("not implemented")
}

type mockClusterRoleTemplateBindingCache struct {
	store map[string]*v3.ClusterRoleTemplateBinding
}

func (m mockClusterRoleTemplateBindingCache) Get(namespace, name string) (*v3.ClusterRoleTemplateBinding, error) {
	return m.store[namespace+":"+name], nil
}

func (m mockClusterRoleTemplateBindingCache) List(_ string, _ labels.Selector) ([]*v3.ClusterRoleTemplateBinding, error) {
	var lst []*v3.ClusterRoleTemplateBinding
	for _, v := range m.store {
		lst = append(lst, v)
	}
	return lst, nil
}

func (m mockClusterRoleTemplateBindingCache) AddIndexer(_ string, _ controllers.ClusterRoleTemplateBindingIndexer) {
	panic("not implemented")
}

func (m mockClusterRoleTemplateBindingCache) GetByIndex(_, _ string) ([]*v3.ClusterRoleTemplateBinding, error) {
	panic("not implemented")
}

type mockClusterRoleTemplateBindingClient struct {
	store map[string]*v3.ClusterRoleTemplateBinding
}

func (m mockClusterRoleTemplateBindingClient) Create(_ *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	panic("not implemented")
}

func (m mockClusterRoleTemplateBindingClient) Update(_ *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	panic("not implemented")
}

func (m mockClusterRoleTemplateBindingClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	delete(m.store, namespace+":"+name)
	return nil
}

func (m mockClusterRoleTemplateBindingClient) Get(_, _ string, _ metav1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
	panic("not implemented")
}

func (m mockClusterRoleTemplateBindingClient) List(_ string, _ metav1.ListOptions) (*v3.ClusterRoleTemplateBindingList, error) {
	panic("not implemented")
}

func (m mockClusterRoleTemplateBindingClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}

func (m mockClusterRoleTemplateBindingClient) Patch(_, _ string, _ apitypes.PatchType, _ []byte, _ ...string) (result *v3.ClusterRoleTemplateBinding, err error) {
	panic("not implemented")
}

type mockUserCache struct {
	store map[string]*v3.User
}

func (m mockUserCache) Get(name string) (*v3.User, error) {
	return m.store[name], nil
}

func (m mockUserCache) List(_ labels.Selector) ([]*v3.User, error) {
	var lst []*v3.User
	for _, v := range m.store {
		lst = append(lst, v)
	}
	return lst, nil
}

func (m mockUserCache) AddIndexer(_ string, _ controllers.UserIndexer) {}

func (m mockUserCache) GetByIndex(_, _ string) ([]*v3.User, error) {
	panic("not implemented")
}

type mockUserClient struct {
	store map[string]*v3.User
}

func (m mockUserClient) Create(_ *v3.User) (*v3.User, error) {
	panic("not implemented")
}

func (m mockUserClient) Update(user *v3.User) (*v3.User, error) {
	m.store[user.Name] = user
	return user, nil
}

func (m mockUserClient) UpdateStatus(_ *v3.User) (*v3.User, error) {
	panic("not implemented")
}

func (m mockUserClient) Delete(name string, _ *metav1.DeleteOptions) error {
	delete(m.store, name)
	return nil
}

func (m mockUserClient) Get(_ string, _ metav1.GetOptions) (*v3.User, error) {
	panic("not implemented")
}

func (m mockUserClient) List(_ metav1.ListOptions) (*v3.UserList, error) {
	panic("not implemented")
}

func (m mockUserClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}

func (m mockUserClient) Patch(_ string, _ apitypes.PatchType, _ []byte, _ ...string) (result *v3.User, err error) {
	panic("not implemented")
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

	secretInterfaceMock.ListNamespacedFunc = func(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error) {
		var secrets []corev1.Secret
		for _, secret := range store {
			if secret.Namespace == namespace {
				secrets = append(secrets, *secret)
			}
		}
		return &corev1.SecretList{
			Items: secrets,
		}, nil
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
