package cleanup

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeExtTokenStore struct {
	tokens    map[string]*ext.Token
	listErr   error
	deleteErr error
}

func (f *fakeExtTokenStore) ListForConfig(configName string) (*ext.TokenList, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var items []ext.Token
	for _, t := range f.tokens {
		if tokenConfigName, err := common.ConfigNameFromToken(t); err == nil && tokenConfigName == configName {
			items = append(items, *t)
		}
	}
	return &ext.TokenList{Items: items}, nil
}

func (f *fakeExtTokenStore) Delete(name string, _ *metav1.DeleteOptions) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.tokens, name)
	return nil
}

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
			ObjectMeta:    metav1.ObjectMeta{Name: "local-123"},
			UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "local://bob"}},
		},
		"azure-123": {
			ObjectMeta:    metav1.ObjectMeta{Name: "azure-123"},
			UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "azuread_user://alice"}},
		},
		"openldap-333": {
			ObjectMeta:    metav1.ObjectMeta{Name: "openldap-333"},
			UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "openldap_user://carl"}},
		},
	}

	var extTokens = map[string]*ext.Token{
		"ext-azure-1": {
			ObjectMeta: metav1.ObjectMeta{Name: "ext-azure-1"},
			Spec: ext.TokenSpec{
				UserPrincipal: ext.TokenPrincipal{Name: "azuread_user://alice", Provider: "azuread"},
			},
		},
		"ext-azure-2": {
			ObjectMeta: metav1.ObjectMeta{Name: "ext-azure-2"},
			Spec: ext.TokenSpec{
				UserPrincipal: ext.TokenPrincipal{Name: "azuread_user://alice", Provider: "azuread"},
			},
		},
		"ext-local-1": {
			ObjectMeta: metav1.ObjectMeta{Name: "ext-local-1"},
			Spec: ext.TokenSpec{
				UserPrincipal: ext.TokenPrincipal{Name: "local://bob", Provider: "local"},
			},
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

	extStore := &fakeExtTokenStore{tokens: extTokens}
	svc := newMockCleanupService(t,
		globalRoleBindingStore,
		projectRoleTemplateBindingStore,
		clusterRoleTemplateBindingStore,
		tokenStore,
		userStore,
		secretStore,
		extStore,
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
	assert.Len(t, extTokens, 1)
	assert.NotNil(t, extTokens["ext-local-1"])

	for _, user := range userStore {
		require.Lenf(t, user.PrincipalIDs, 1, "every user after cleanup must have only one principal ID, got %d", len(user.PrincipalIDs))
		principalID := user.PrincipalIDs[0]
		assert.Truef(t, strings.HasPrefix(principalID, "local"), "the only principal ID has 'local' as a prefix, got %s", principalID)
	}
}

// TestRunCleanupTokensSelectedByConfigName verifies that when several AuthConfigs
// share a provider type, disabling one of them only deletes the tokens issued by
// that config. Tokens are selected by the config name prefix of their user
// principal.
func TestRunCleanupTokensSelectedByConfigName(t *testing.T) {
	newToken := func(name, principalID string) *v3.Token {
		return &v3.Token{
			ObjectMeta:    metav1.ObjectMeta{Name: name},
			UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: principalID}, Provider: "github"},
		}
	}
	newExtToken := func(name, principalID string) *ext.Token {
		return &ext.Token{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: ext.TokenSpec{
				UserPrincipal: ext.TokenPrincipal{Name: principalID, Provider: "github"},
			},
		}
	}

	tests := []struct {
		configName        string
		wantTokens        []string
		wantExtTokens     []string
		wantUsers         []string
		wantUserPrincipal map[string][]string
	}{
		{
			configName: "github-eu",
			// carol also has a local password, so the user is kept and reset
			// to local. Her github-eu tokens must not survive the cleanup,
			// otherwise they work again when github-eu is re-enabled.
			wantTokens:    []string{"gh-alice", "local-bob", "agent-x"},
			wantExtTokens: []string{"ext-gh-alice", "ext-local-bob", "ext-agent-x"},
			wantUsers:     []string{"alice", "bob", "carol"},
			wantUserPrincipal: map[string][]string{
				"alice": {"github_user://1"},
				"carol": {"local://carol"},
			},
		},
		{
			configName:    "github",
			wantTokens:    []string{"gh-eu-carol", "gh-eu-dave", "local-bob", "agent-x"},
			wantExtTokens: []string{"ext-gh-eu-carol", "ext-gh-eu-dave", "ext-local-bob", "ext-agent-x"},
			wantUsers:     []string{"bob", "carol", "dave"},
			wantUserPrincipal: map[string][]string{
				"carol": {"github-eu_user://3", "local://carol"},
				"dave":  {"github-eu_user://4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run("disable "+tt.configName, func(t *testing.T) {
			userStore := map[string]*v3.User{
				"alice": {
					ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
					PrincipalIDs: []string{"github_user://1"},
				},
				"bob": {
					ObjectMeta:   metav1.ObjectMeta{Name: "bob"},
					PrincipalIDs: []string{"local://bob"},
				},
				"carol": {
					ObjectMeta:   metav1.ObjectMeta{Name: "carol"},
					PrincipalIDs: []string{"github-eu_user://3", "local://carol"},
					Password:     "secret",
				},
				"dave": {
					ObjectMeta:   metav1.ObjectMeta{Name: "dave"},
					PrincipalIDs: []string{"github-eu_user://4"},
				},
			}

			localToken := &v3.Token{
				ObjectMeta:    metav1.ObjectMeta{Name: "local-bob"},
				UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "local://bob"}, Provider: "local"},
			}
			tokenStore := map[string]*v3.Token{
				"gh-alice":    newToken("gh-alice", "github_user://1"),
				"gh-eu-carol": newToken("gh-eu-carol", "github-eu_user://3"),
				"gh-eu-dave":  newToken("gh-eu-dave", "github-eu_user://4"),
				"local-bob":   localToken,
				// Rancher-issued controller tokens (e.g. autoscaler, compose)
				// have no user principal and belong to local; they must
				// survive the cleanup of any other AuthConfig.
				"agent-x": {
					ObjectMeta:   metav1.ObjectMeta{Name: "agent-x"},
					AuthProvider: "local",
				},
			}

			extTokens := map[string]*ext.Token{
				"ext-gh-alice":    newExtToken("ext-gh-alice", "github_user://1"),
				"ext-gh-eu-carol": newExtToken("ext-gh-eu-carol", "github-eu_user://3"),
				"ext-gh-eu-dave":  newExtToken("ext-gh-eu-dave", "github-eu_user://4"),
				"ext-local-bob": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-local-bob"},
					Spec: ext.TokenSpec{
						UserPrincipal: ext.TokenPrincipal{Name: "local://bob", Provider: "local"},
					},
				},
				"ext-agent-x": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-agent-x"},
				},
			}

			svc := newMockCleanupService(t,
				map[string]*v3.GlobalRoleBinding{},
				map[string]*v3.ProjectRoleTemplateBinding{},
				map[string]*v3.ClusterRoleTemplateBinding{},
				tokenStore,
				userStore,
				map[string]*v1.Secret{},
				&fakeExtTokenStore{tokens: extTokens},
			)

			err := svc.Run(&v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: tt.configName},
				Type:       client.GithubConfigType,
			})
			require.NoError(t, err)

			assert.ElementsMatch(t, tt.wantUsers, slices.Collect(maps.Keys(userStore)), "users")
			for name, principalIDs := range tt.wantUserPrincipal {
				assert.ElementsMatch(t, principalIDs, userStore[name].PrincipalIDs, "principal IDs of %s", name)
			}
			assert.ElementsMatch(t, tt.wantTokens, slices.Collect(maps.Keys(tokenStore)), "tokens")
			assert.ElementsMatch(t, tt.wantExtTokens, slices.Collect(maps.Keys(extTokens)), "ext tokens")
		})
	}
}

func newMockCleanupService(t *testing.T,
	grbStore map[string]*v3.GlobalRoleBinding,
	prtbStore map[string]*v3.ProjectRoleTemplateBinding,
	crtbStore map[string]*v3.ClusterRoleTemplateBinding,
	tokenStore map[string]*v3.Token,
	userStore map[string]*v3.User,
	secretStore map[string]*v1.Secret,
	extStore extTokenStore) Service {
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
	impUserClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
	userClient.EXPECT().WithImpersonation(gomock.Any()).Return(impUserClient, nil).AnyTimes()
	impUserClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(user *v3.User) (*v3.User, error) {
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

	// Setup Group mock client
	groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
	groupClient.EXPECT().List(gomock.Any()).DoAndReturn(func(_ metav1.ListOptions) (*v3.GroupList, error) {
		return &v3.GroupList{}, nil
	}).AnyTimes()

	// Setup SecretsCache mock client, which reads the same store as the
	// secrets client.
	secretsCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretsCache.EXPECT().Get(pbkdf2.LocalUserPasswordsNamespace, gomock.Any()).DoAndReturn(func(namespace, name string) (*corev1.Secret, error) {
		if secret, ok := secretStore[namespace+":"+name]; ok {
			return secret, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    v1.SchemeGroupVersion.Group,
			Resource: "secrets",
		}, name)
	}).AnyTimes()

	return Service{
		secretsInterface:                  getSecretControllerMock(ctrl, secretStore),
		secretsCache:                      secretsCache,
		groupClient:                       groupClient,
		globalRoleBindingsCache:           grbCache,
		globalRoleBindingsClient:          grbClient,
		projectRoleTemplateBindingsCache:  prtbCache,
		projectRoleTemplateBindingsClient: prtbClient,
		clusterRoleTemplateBindingsCache:  crtbCache,
		clusterRoleTemplateBindingsClient: crtbClient,
		tokensCache:                       tokenCache,
		tokensClient:                      tokenClient,
		userClient:                        userClient,
		extTokenStore:                     extStore,
	}
}

func getSecretControllerMock(ctrl *gomock.Controller, store map[string]*corev1.Secret) wcorev1.SecretController {
	secretController := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretController.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		if secret.Name == "" {
			uniqueIdentifier := md5.Sum([]byte(time.Now().String()))
			secret.Name = hex.EncodeToString(uniqueIdentifier[:])
		}
		store[fmt.Sprintf("%s:%s", secret.Namespace, secret.Name)] = secret
		return secret, nil
	}).AnyTimes()

	secretController.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error) {
		var secrets []corev1.Secret
		for _, secret := range store {
			if secret.Namespace == namespace {
				secrets = append(secrets, *secret)
			}
		}
		return &corev1.SecretList{
			Items: secrets,
		}, nil
	}).AnyTimes()

	secretController.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
		secret, ok := store[fmt.Sprintf("%s:%s", namespace, name)]
		if ok {
			return secret, nil
		}
		return nil, errors.New("secret not found")
	}).AnyTimes()

	secretController.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, options *metav1.DeleteOptions) error {
		delete(store, fmt.Sprintf("%s:%s", namespace, name))
		return nil
	}).AnyTimes()

	return secretController
}

func TestDeleteExtTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		provider  string
		tokens    map[string]*ext.Token
		listErr   error
		deleteErr error
		wantErr   bool
		wantLeft  []string
	}{
		{
			name:     "no matching tokens",
			provider: "azuread",
			tokens: map[string]*ext.Token{
				"ext-local": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-local"},
					Spec:       ext.TokenSpec{UserPrincipal: ext.TokenPrincipal{Name: "local://bob", Provider: "local"}},
				},
			},
			wantLeft: []string{"ext-local"},
		},
		{
			name:     "deletes only matching config tokens",
			provider: "azuread",
			tokens: map[string]*ext.Token{
				"ext-azure": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-azure"},
					Spec:       ext.TokenSpec{UserPrincipal: ext.TokenPrincipal{Name: "azuread_user://alice", Provider: "azuread"}},
				},
				"ext-local": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-local"},
					Spec:       ext.TokenSpec{UserPrincipal: ext.TokenPrincipal{Name: "local://bob", Provider: "local"}},
				},
				"ext-ping": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-ping"},
					Spec:       ext.TokenSpec{UserPrincipal: ext.TokenPrincipal{Name: "ping_user://dan", Provider: "ping"}},
				},
			},
			wantLeft: []string{"ext-local", "ext-ping"},
		},
		{
			name:     "list error propagates",
			provider: "azuread",
			tokens:   map[string]*ext.Token{},
			listErr:  errors.New("list failed"),
			wantErr:  true,
		},
		{
			name:     "delete error propagates",
			provider: "azuread",
			tokens: map[string]*ext.Token{
				"ext-azure": {
					ObjectMeta: metav1.ObjectMeta{Name: "ext-azure"},
					Spec:       ext.TokenSpec{UserPrincipal: ext.TokenPrincipal{Name: "azuread_user://alice", Provider: "azuread"}},
				},
			},
			deleteErr: errors.New("delete failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeExtTokenStore{
				tokens:    tt.tokens,
				listErr:   tt.listErr,
				deleteErr: tt.deleteErr,
			}
			svc := &Service{extTokenStore: store}
			err := svc.deleteExtTokens(tt.provider)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, store.tokens, len(tt.wantLeft))
			for _, name := range tt.wantLeft {
				assert.Contains(t, store.tokens, name)
			}
		})
	}
}

func TestConfigNameFromPrincipalNames(t *testing.T) {
	tests := map[string]struct {
		names   []string
		want    string
		wantErr string
	}{
		"config of a user principal": {
			names: []string{"openldap-corp_user://cn=alice,dc=corp"},
			want:  "openldap-corp",
		},
		"config whose name starts with local": {
			names: []string{"local-ldap_group://cn=admins"},
			want:  "local-ldap",
		},
		"empty, local and system principals are skipped": {
			names: []string{"", "local://u-1", "system://c-1", "github-eu_user://1"},
			want:  "github-eu",
		},
		"the first config wins": {
			names: []string{"ldap-corp_user://a", "ldap-it_user://b"},
			want:  "ldap-corp",
		},
		"no principal issued by a config": {
			names: []string{"local://u-1", "system://provisioning/fleet-default/c1", ""},
		},
		"no principals": {},
		"unparseable principal": {
			names:   []string{"garbage"},
			wantErr: "invalid principal id garbage",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := configNameFromPrincipalNames(tt.names...)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRunCleanupConfigNamedLikeLocal verifies that the bindings and users of
// an AuthConfig whose name starts with "local" are cleaned up, and that
// principals that can't be parsed are left alone.
func TestRunCleanupConfigNamedLikeLocal(t *testing.T) {
	globalRoleBindingStore := map[string]*v3.GlobalRoleBinding{
		"local-ldap": {
			ObjectMeta:         metav1.ObjectMeta{Name: "local-ldap"},
			GroupPrincipalName: "local-ldap_group://cn=admins",
		},
		"garbage": {
			ObjectMeta:         metav1.ObjectMeta{Name: "garbage"},
			GroupPrincipalName: "garbage",
		},
	}
	projectRoleTemplateBindingStore := map[string]*v3.ProjectRoleTemplateBinding{
		"p1:local-ldap": {
			ObjectMeta:        metav1.ObjectMeta{Name: "local-ldap", Namespace: "p1"},
			UserPrincipalName: "local-ldap_user://cn=alice",
		},
		"p1:local": {
			ObjectMeta:        metav1.ObjectMeta{Name: "local", Namespace: "p1"},
			UserPrincipalName: "local://u-bob",
		},
	}
	clusterRoleTemplateBindingStore := map[string]*v3.ClusterRoleTemplateBinding{
		"c1:local-ldap": {
			ObjectMeta:         metav1.ObjectMeta{Name: "local-ldap", Namespace: "c1"},
			GroupPrincipalName: "local-ldap_group://cn=admins",
		},
		"c1:system": {
			ObjectMeta:        metav1.ObjectMeta{Name: "system", Namespace: "c1"},
			UserPrincipalName: "system://c1",
		},
	}
	userStore := map[string]*v3.User{
		"alice": {
			ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
			PrincipalIDs: []string{"local-ldap_user://cn=alice"},
		},
		"bob": {
			ObjectMeta:   metav1.ObjectMeta{Name: "bob"},
			PrincipalIDs: []string{"local://u-bob"},
		},
		"system": {
			ObjectMeta:   metav1.ObjectMeta{Name: "system"},
			PrincipalIDs: []string{"system://c1"},
		},
		"garbage": {
			ObjectMeta:   metav1.ObjectMeta{Name: "garbage"},
			PrincipalIDs: []string{"garbage"},
		},
	}

	svc := newMockCleanupService(t,
		globalRoleBindingStore,
		projectRoleTemplateBindingStore,
		clusterRoleTemplateBindingStore,
		map[string]*v3.Token{},
		userStore,
		map[string]*v1.Secret{},
		&fakeExtTokenStore{tokens: map[string]*ext.Token{}},
	)
	cfg := v3.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "local-ldap"},
		Type:       client.OpenLdapConfigType,
	}

	require.NoError(t, svc.Run(&cfg))

	assert.Equal(t, []string{"garbage"}, slices.Sorted(maps.Keys(globalRoleBindingStore)))
	assert.Equal(t, []string{"p1:local"}, slices.Sorted(maps.Keys(projectRoleTemplateBindingStore)))
	assert.Equal(t, []string{"c1:system"}, slices.Sorted(maps.Keys(clusterRoleTemplateBindingStore)))
	assert.Equal(t, []string{"bob", "garbage", "system"}, slices.Sorted(maps.Keys(userStore)))
}

// TestRunCleanupGlobalRoleBindingsOfUserPrincipal verifies that disabling an
// AuthConfig deletes the global role bindings of its user principals, not
// only those of its group principals. A user who also has principals of
// other AuthConfigs is kept, so their bindings aren't removed along with them
// and must be deleted here.
func TestRunCleanupGlobalRoleBindingsOfUserPrincipal(t *testing.T) {
	globalRoleBindingStore := map[string]*v3.GlobalRoleBinding{
		"corp-user": {
			ObjectMeta:        metav1.ObjectMeta{Name: "corp-user"},
			UserName:          "alice",
			UserPrincipalName: "ldap-corp_user://a",
			GlobalRoleName:    "admin",
		},
		"corp-group": {
			ObjectMeta:         metav1.ObjectMeta{Name: "corp-group"},
			GroupPrincipalName: "ldap-corp_group://admins",
			GlobalRoleName:     "admin",
		},
		"it-user": {
			ObjectMeta:        metav1.ObjectMeta{Name: "it-user"},
			UserName:          "alice",
			UserPrincipalName: "ldap-it_user://b",
			GlobalRoleName:    "user",
		},
		"local-user": {
			ObjectMeta:        metav1.ObjectMeta{Name: "local-user"},
			UserName:          "alice",
			UserPrincipalName: "local://u-alice",
			GlobalRoleName:    "user",
		},
		"user-name-only": {
			ObjectMeta:     metav1.ObjectMeta{Name: "user-name-only"},
			UserName:       "alice",
			GlobalRoleName: "user",
		},
	}
	userStore := map[string]*v3.User{
		"alice": {
			ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
			PrincipalIDs: []string{"ldap-corp_user://a", "ldap-it_user://b", "local://u-alice"},
		},
	}

	svc := newMockCleanupService(t,
		globalRoleBindingStore,
		map[string]*v3.ProjectRoleTemplateBinding{},
		map[string]*v3.ClusterRoleTemplateBinding{},
		map[string]*v3.Token{},
		userStore,
		map[string]*v1.Secret{},
		&fakeExtTokenStore{tokens: map[string]*ext.Token{}},
	)

	require.NoError(t, svc.Run(&v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "ldap-corp"}, Type: client.OpenLdapConfigType}))

	assert.Equal(t, []string{"it-user", "local-user", "user-name-only"}, slices.Sorted(maps.Keys(globalRoleBindingStore)))
	require.Contains(t, userStore, "alice", "the user was deleted")
	assert.Equal(t, []string{"ldap-it_user://b", "local://u-alice"}, userStore["alice"].PrincipalIDs)
}

// runUserCleanup disables the named AuthConfig with only the given users in
// the store and returns the store afterwards.
func runUserCleanup(t *testing.T, configName string, users ...*v3.User) map[string]*v3.User {
	t.Helper()

	return runUserCleanupWithSecrets(t, configName, map[string]*v1.Secret{}, users...)
}

// runUserCleanupWithSecrets is runUserCleanup with the given Secrets, keyed by
// "namespace:name", in the store.
func runUserCleanupWithSecrets(t *testing.T, configName string, secretStore map[string]*v1.Secret, users ...*v3.User) map[string]*v3.User {
	t.Helper()

	userStore := map[string]*v3.User{}
	for _, u := range users {
		userStore[u.Name] = u
	}
	svc := newMockCleanupService(t,
		map[string]*v3.GlobalRoleBinding{},
		map[string]*v3.ProjectRoleTemplateBinding{},
		map[string]*v3.ClusterRoleTemplateBinding{},
		map[string]*v3.Token{},
		userStore,
		secretStore,
		&fakeExtTokenStore{tokens: map[string]*ext.Token{}},
	)

	require.NoError(t, svc.Run(&v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: configName}, Type: client.OpenLdapConfigType}))

	return userStore
}

// TestRunCleanupResetLocalUserKeepsLocalPrincipal verifies that resetting a
// user who can log in locally keeps the principal of the local AuthConfig,
// not one of an AuthConfig whose name starts with "local".
func TestRunCleanupResetLocalUserKeepsLocalPrincipal(t *testing.T) {
	users := runUserCleanup(t, "localldap", &v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
		PrincipalIDs: []string{"localldap_user://cn=alice", "local://u-alice"},
		Password:     "secret",
	})

	require.Contains(t, users, "alice")
	assert.Equal(t, []string{"local://u-alice"}, users["alice"].PrincipalIDs)
}

// TestRunCleanupUserWithSeveralConfigs verifies that disabling an AuthConfig
// only removes that AuthConfig's principals from a user who also has
// principals of other AuthConfigs, and only deletes the user when nothing
// they can log in with is left.
func TestRunCleanupUserWithSeveralConfigs(t *testing.T) {
	tests := map[string]struct {
		configName     string
		user           *v3.User
		wantPrincipals []string // nil if the user should be deleted
	}{
		"disabling the config of the first principal keeps the other config's principal": {
			configName: "ldap-corp",
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"ldap-corp_user://a", "ldap-it_user://b"},
			},
			wantPrincipals: []string{"ldap-it_user://b"},
		},
		"disabling the config of a later principal removes that principal": {
			configName: "ldap-it",
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"ldap-corp_user://a", "ldap-it_user://b"},
			},
			wantPrincipals: []string{"ldap-corp_user://a"},
		},
		"a local user keeps the principals of other configs": {
			configName: "ldap-corp",
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"local://u-alice", "ldap-corp_user://a", "ldap-it_user://b"},
				Password:     "secret",
			},
			wantPrincipals: []string{"local://u-alice", "ldap-it_user://b"},
		},
		"a user with only the disabled config's principals is deleted": {
			configName: "ldap-corp",
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"ldap-corp_user://a", "ldap-corp_user://a2"},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			users := runUserCleanup(t, tt.configName, tt.user)

			if tt.wantPrincipals == nil {
				assert.NotContains(t, users, "alice")
				return
			}
			require.Contains(t, users, "alice", "the user was deleted")
			assert.Equal(t, tt.wantPrincipals, users["alice"].PrincipalIDs)
		})
	}
}

// TestRunCleanupUserWithPasswordSecret verifies that a user whose local
// password is stored in a Secret, rather than in User.Password, counts as able
// to log in locally and so is not deleted.
func TestRunCleanupUserWithPasswordSecret(t *testing.T) {
	passwordSecret := func(userName string) map[string]*v1.Secret {
		return map[string]*v1.Secret{
			pbkdf2.LocalUserPasswordsNamespace + ":" + userName: {
				ObjectMeta: metav1.ObjectMeta{Namespace: pbkdf2.LocalUserPasswordsNamespace, Name: userName},
			},
		}
	}

	tests := map[string]struct {
		secrets        map[string]*v1.Secret
		user           *v3.User
		wantPrincipals []string // nil if the user should be deleted
	}{
		"a user with a password Secret is kept with their local principal": {
			secrets: passwordSecret("alice"),
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"local://u-alice", "ldap-corp_user://a"},
			},
			wantPrincipals: []string{"local://u-alice"},
		},
		"a user with a password Secret and only the disabled config's principal is left untouched": {
			secrets: passwordSecret("alice"),
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"ldap-corp_user://a"},
			},
			wantPrincipals: []string{"ldap-corp_user://a"},
		},
		"another user's password Secret doesn't keep the user": {
			secrets: passwordSecret("bob"),
			user: &v3.User{
				ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
				PrincipalIDs: []string{"local://u-alice", "ldap-corp_user://a"},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			users := runUserCleanupWithSecrets(t, "ldap-corp", tt.secrets, tt.user)

			if tt.wantPrincipals == nil {
				assert.NotContains(t, users, "alice")
				return
			}
			require.Contains(t, users, "alice", "the user was deleted")
			assert.Equal(t, tt.wantPrincipals, users["alice"].PrincipalIDs)
		})
	}
}

func TestRunCleanupLocalConfigKeepsUsers(t *testing.T) {
	users := runUserCleanup(t, "local", &v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "alice"},
		PrincipalIDs: []string{"local://u-alice"},
	})

	require.Contains(t, users, "alice")
	assert.Equal(t, []string{"local://u-alice"}, users["alice"].PrincipalIDs)
}
