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
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
		"boss": {
			ObjectMeta: metav1.ObjectMeta{
				Name:   "boss",
				Labels: map[string]string{"authz.management.cattle.io/bootstrapping": "admin-user"}},
			PrincipalIDs: []string{"local://boss", "azuread_user://authprincipal"},
		},
	}

	var tokenStore = map[string]*v3.Token{
		"local-123": {
			ObjectMeta:   metav1.ObjectMeta{Name: "local-123"},
			AuthProvider: "local",
		},
		"azure-123": {
			ObjectMeta:   metav1.ObjectMeta{Name: "azure-123"},
			AuthProvider: "azuread",
		},
		"openldap-333": {
			ObjectMeta:   metav1.ObjectMeta{Name: "openldap-333"},
			AuthProvider: "openldap",
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

	svc := newMockCleanupService(t,
		globalRoleBindingStore,
		projectRoleTemplateBindingStore,
		clusterRoleTemplateBindingStore,
		tokenStore,
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
	assert.Len(t, tokenStore, 2)
	assert.Empty(t, tokenStore["azure-123"])

	for _, user := range userStore {
		require.Lenf(t, user.PrincipalIDs, 1, "every user after cleanup must have only one principal ID, got %d", len(user.PrincipalIDs))
		principalID := user.PrincipalIDs[0]
		assert.Truef(t, strings.HasPrefix(principalID, "local"), "the only principal ID has 'local' as a prefix, got %s", principalID)
	}
}

func newMockCleanupService(t *testing.T,
	grbStore map[string]*v3.GlobalRoleBinding,
	prtbStore map[string]*v3.ProjectRoleTemplateBinding,
	crtbStore map[string]*v3.ClusterRoleTemplateBinding,
	tokenStore map[string]*v3.Token,
	userStore map[string]*v3.User,
	secretStore map[string]*v1.Secret) Service {
	t.Helper()
	ctrl := gomock.NewController(t)

	// Setup GlobalRole mock cache
	grbCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().List(gomock.Any()).DoAndReturn(func(_ labels.Selector) ([]*v3.GlobalRoleBinding, error) {
		var lst []*v3.GlobalRoleBinding
		for _, v := range grbStore {
			lst = append(lst, v)
		}
		return lst, nil
	}).AnyTimes()
	grbCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.GlobalRoleBinding, error) {
		return grbStore[name], nil
	}).AnyTimes()

	// Setup GlobalRole mock client
	grbClient := fake.NewMockNonNamespacedClientInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](ctrl)
	grbClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ *metav1.DeleteOptions) error {
		delete(grbStore, name)
		return nil
	}).AnyTimes()

	// Setup ProjectRoleTemplateBinding mock cache
	prtbCache := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
	prtbCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, _ labels.Selector) ([]*v3.ProjectRoleTemplateBinding, error) {
		var lst []*v3.ProjectRoleTemplateBinding
		for _, v := range prtbStore {
			lst = append(lst, v)
		}
		return lst, nil
	}).AnyTimes()
	prtbCache.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string) (*v3.ProjectRoleTemplateBinding, error) {
		return prtbStore[namespace+":"+name], nil
	}).AnyTimes()

	// Setup ProjectRoleTemplateBinding mock client
	prtbClient := fake.NewMockClientInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
	prtbClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, _ *metav1.DeleteOptions) error {
		delete(prtbStore, namespace+":"+name)
		return nil
	}).AnyTimes()

	// Setup ClusterRoleTemplateBinding mock cache
	crtbCache := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
	crtbCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, _ labels.Selector) ([]*v3.ClusterRoleTemplateBinding, error) {
		var lst []*v3.ClusterRoleTemplateBinding
		for _, v := range crtbStore {
			lst = append(lst, v)
		}
		return lst, nil
	}).AnyTimes()
	crtbCache.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string) (*v3.ClusterRoleTemplateBinding, error) {
		return crtbStore[namespace+":"+name], nil
	}).AnyTimes()

	// Setup ClusterRoleTemplateBinding mock client
	crtbClient := fake.NewMockClientInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
	crtbClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, _ *metav1.DeleteOptions) error {
		delete(crtbStore, namespace+":"+name)
		return nil
	}).AnyTimes()

	// Setup Token mock cache
	tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
	tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(_ labels.Selector) ([]*v3.Token, error) {
		var lst []*v3.Token
		for _, v := range tokenStore {
			lst = append(lst, v)
		}
		return lst, nil
	}).AnyTimes()
	tokenCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.Token, error) {
		return tokenStore[name], nil
	}).AnyTimes()

	// Setup Token mock client
	tokenClient := fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](ctrl)
	tokenClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ *metav1.DeleteOptions) error {
		delete(tokenStore, name)
		return nil
	}).AnyTimes()

	// Setup User mock client
	userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
	userClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ *metav1.DeleteOptions) error {
		delete(userStore, name)
		return nil
	}).AnyTimes()
	userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(user *v3.User) (*v3.User, error) {
		userStore[user.Name] = user
		return user, nil
	}).AnyTimes()
	userClient.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.UserList, error) {
		var lst v3.UserList
		for _, v := range userStore {
			selector, err := labels.Parse(opts.LabelSelector)
			if err != nil {
				return nil, err
			}
			if selector.Matches(labels.Set(v.Labels)) {
				lst.Items = append(lst.Items, *v)
			}
		}
		return &lst, nil
	}).AnyTimes()

	return Service{
		secretsInterface:                  getSecretInterfaceMock(secretStore),
		globalRoleBindingsCache:           grbCache,
		globalRoleBindingsClient:          grbClient,
		projectRoleTemplateBindingsCache:  prtbCache,
		projectRoleTemplateBindingsClient: prtbClient,
		clusterRoleTemplateBindingsCache:  crtbCache,
		clusterRoleTemplateBindingsClient: crtbClient,
		tokensCache:                       tokenCache,
		tokensClient:                      tokenClient,
		userClient:                        userClient,
	}
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
