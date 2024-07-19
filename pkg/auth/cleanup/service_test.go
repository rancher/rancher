package cleanup

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	generic "github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	}

	var tokensStore = map[string]*v3.Token{
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
		tokensStore,
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

func newMockCleanupService(t *testing.T,
	grbStore map[string]*v3.GlobalRoleBinding,
	prtbStore map[string]*v3.ProjectRoleTemplateBinding,
	crtbStore map[string]*v3.ClusterRoleTemplateBinding,
	tokenStore map[string]*v3.Token,
	userStore map[string]*v3.User,
	secretStore map[string]*v1.Secret) Service {
	t.Helper()
	ctrl := gomock.NewController(t)

	grbCache := initMockCache(ctrl, grbStore)
	grbClient := initMockClient[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](ctrl, grbStore)

	prtbCache := initNamespacedMockCache(ctrl, prtbStore)
	prtbClient := initNamespacedMockClient[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl, prtbStore)

	crtbCache := initNamespacedMockCache(ctrl, crtbStore)
	crtbClient := initNamespacedMockClient[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl, crtbStore)

	tokenCache := initMockCache(ctrl, tokenStore)
	tokenClient := initMockClient[*v3.Token, *v3.TokenList](ctrl, tokenStore)

	userCache := initMockCache(ctrl, userStore)
	userClient := initMockClient[*v3.User, *v3.UserList](ctrl, userStore)
	userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.User) (*v3.User, error) {
		userStore[obj.GetName()] = obj
		return obj, nil
	})

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
		userCache:                         userCache,
		userClient:                        userClient,
	}
}

// initMockCache will set up the mock with an expectation on the List(name string) and Get(name string)
func initMockCache[T runtime.Object](ctrl *gomock.Controller, store map[string]T) *fake.MockNonNamespacedCacheInterface[T] {
	cache := fake.NewMockNonNamespacedCacheInterface[T](ctrl)

	cache.EXPECT().List(gomock.Any()).DoAndReturn(func(_ labels.Selector) ([]T, error) {
		var lst []T
		for _, v := range store {
			lst = append(lst, v)
		}
		return lst, nil
	}).AnyTimes()

	cache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (T, error) {
		return store[name], nil
	}).AnyTimes()

	return cache
}

// initNamespacedMockCache will set up the mock with an expectation on the List(name, namespace string) and Get(name, namespace string)
// The Get is returning the object from the store with the "name:namespace" key
func initNamespacedMockCache[T runtime.Object](ctrl *gomock.Controller, store map[string]T) *fake.MockCacheInterface[T] {
	cache := fake.NewMockCacheInterface[T](ctrl)

	cache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, _ labels.Selector) ([]T, error) {
		var lst []T
		for _, v := range store {
			lst = append(lst, v)
		}
		return lst, nil
	}).AnyTimes()

	cache.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string) (T, error) {
		return store[namespace+":"+name], nil
	}).AnyTimes()

	return cache
}

// initMockClient will set up the mock with an expectation on the Delete(name string)
func initMockClient[T generic.RuntimeMetaObject, L runtime.Object](ctrl *gomock.Controller, store map[string]T) *fake.MockNonNamespacedClientInterface[T, L] {
	cl := fake.NewMockNonNamespacedClientInterface[T, L](ctrl)

	cl.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ *metav1.DeleteOptions) error {
		delete(store, name)
		return nil
	}).AnyTimes()

	return cl
}

// initNamespacedMockClient will set up the mock with an expectation on the Delete(name, namespace string)
// The Delete will remove the object from the store with the "name:namespace" key
func initNamespacedMockClient[T generic.RuntimeMetaObject, L runtime.Object](ctrl *gomock.Controller, store map[string]T) *fake.MockClientInterface[T, L] {
	cl := fake.NewMockClientInterface[T, L](ctrl)

	cl.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, _ *metav1.DeleteOptions) error {
		delete(store, namespace+":"+name)
		return nil
	}).AnyTimes()

	return cl
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
