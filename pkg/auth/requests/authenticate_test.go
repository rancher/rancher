package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/clusterrouter"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	mgmtFakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
)

type fakeUserRefresher struct {
	called bool
	userID string
	force  bool
}

func (r *fakeUserRefresher) refreshUser(userID string, force bool) {
	r.called = true
	r.userID = userID
	r.force = force
}

func (r *fakeUserRefresher) reset() {
	r.called = false
	r.userID = ""
	r.force = false
}

type fakeProvider struct {
	name     string
	disabled bool
}

func (p *fakeProvider) IsDisabledProvider() (bool, error) {
	return p.disabled, nil
}

func (p *fakeProvider) GetName() string {
	return p.name
}

func (p *fakeProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *fakeProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *fakeProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	panic("not implemented")
}

func (p *fakeProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *fakeProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *fakeProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *fakeProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	panic("not implemented")
}

func (p *fakeProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: {userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    {userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *fakeProvider) CleanupResources(*v3.AuthConfig) error {
	return nil
}

func TestTokenAuthenticatorAuthenticate(t *testing.T) {
	existingProviders := providers.Providers
	defer func() {
		providers.Providers = existingProviders
	}()

	fakeProvider := &fakeProvider{
		name: "fake",
	}
	providers.Providers = map[string]common.AuthProvider{
		fakeProvider.name: fakeProvider,
	}

	now := time.Now()
	userID := "u-abcdef"
	token := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "token-v2rcx",
			CreationTimestamp: metav1.NewTime(now),
		},
		Token:        "jnb9tksmnctvgbn92ngbkptblcjwg4pmfp98wqj29wk5kv85ktg59s",
		AuthProvider: fakeProvider.name,
		TTLMillis:    57600000,
		UserID:       userID,
		UserPrincipal: v3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: userID,
			},
		},
	}

	mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	mockIndexer.AddIndexers(cache.Indexers{tokenKeyIndex: tokenKeyIndexer})
	mockIndexer.Add(token)

	userAttributeLister := &mgmtFakes.UserAttributeListerMock{
		GetFunc: func(namespace, name string) (*v3.UserAttribute, error) {
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{
					Name: userID,
				},
				GroupPrincipals: map[string]apiv3.Principals{
					fakeProvider.name: {
						Items: []apiv3.Principal{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: fakeProvider.name + "_group://56789",
								},
								MemberOf:      true,
								LoginName:     "rancher",
								DisplayName:   "rancher",
								PrincipalType: "group",
								Provider:      fakeProvider.name,
							},
						},
					},
				},
				ExtraByProvider: map[string]map[string][]string{
					fakeProvider.name: {
						common.UserAttributePrincipalID: {fakeProvider.name + "_user://12345"},
						common.UserAttributeUserName:    {"fake-user"},
					},
					providers.LocalProvider: {
						common.UserAttributePrincipalID: {"local://" + userID},
						common.UserAttributeUserName:    {"local-user"},
					},
				},
			}, nil
		},
	}

	userLister := &mgmtFakes.UserListerMock{
		GetFunc: func(namespace, name string) (*v3.User, error) {
			return &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: userID,
				},
			}, nil
		},
	}

	userRefresher := &fakeUserRefresher{}

	authenticator := tokenAuthenticator{
		ctx:                 context.Background(),
		tokenIndexer:        mockIndexer,
		userAttributeLister: userAttributeLister,
		userLister:          userLister,
		clusterRouter:       clusterrouter.GetClusterID,
		refreshUser:         userRefresher.refreshUser,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/namespaces", nil)
	req.Header.Set("Authorization", "Bearer "+token.Name+":"+token.Token)

	t.Run("authenticate", func(t *testing.T) {
		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
		assert.Equal(t, userID, resp.User)
		assert.Equal(t, userID, resp.UserPrincipal)
		assert.Contains(t, resp.Groups, fakeProvider.name+"_group://56789")
		assert.Contains(t, resp.Groups, "system:cattle:authenticated")
		assert.Contains(t, resp.Extras[common.UserAttributePrincipalID], fakeProvider.name+"_user://12345")
		assert.Contains(t, resp.Extras[common.UserAttributeUserName], "fake-user")
		assert.True(t, userRefresher.called)
		assert.Equal(t, userID, userRefresher.userID)
		assert.False(t, userRefresher.force)
	})

	t.Run("authenticate with a cluster specific token", func(t *testing.T) {
		clusterID := "c-955nj"
		oldTokenClusterName := token.ClusterName
		defer func() { token.ClusterName = oldTokenClusterName }()
		token.ClusterName = clusterID

		clusterReq := httptest.NewRequest(http.MethodGet, "/k8s/clusters/"+clusterID+"/v1/management.cattle.io.authconfigs", nil)
		clusterReq.Header.Set("Authorization", "Bearer "+token.Name+":"+token.Token)

		userRefresher.reset()

		resp, err := authenticator.Authenticate(clusterReq)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
		assert.True(t, userRefresher.called)
	})

	t.Run("authenticate if userattribute doesn't exist", func(t *testing.T) {
		oldGetUserAttributeFunc := userAttributeLister.GetFunc
		defer func() { userAttributeLister.GetFunc = oldGetUserAttributeFunc }()
		userAttributeLister.GetFunc = func(namespace, name string) (*v3.UserAttribute, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
		assert.True(t, userRefresher.called)
	})

	t.Run("provider refresh is not called if token user id has system prefix", func(t *testing.T) {
		oldTokenUserID := token.UserID
		defer func() { token.UserID = oldTokenUserID }()
		token.UserID = "system://provisioning/fleet-local/local"

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
		assert.False(t, userRefresher.called)
	})

	t.Run("provider refresh is not called for system users", func(t *testing.T) {
		oldGetUserFunc := userLister.GetFunc
		defer func() { userLister.GetFunc = oldGetUserFunc }()
		userLister.GetFunc = func(namespace, name string) (*v3.User, error) {
			return &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: userID,
				},
				PrincipalIDs: []string{
					"system://provisioning/fleet-local/local",
					"local://" + userID,
				},
			}, nil
		}

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
		assert.False(t, userRefresher.called)
	})

	t.Run("token is disabled", func(t *testing.T) {
		oldTokenEnabled := token.Enabled
		defer func() { token.Enabled = oldTokenEnabled }()
		token.Enabled = pointer.BoolPtr(false)

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
		assert.False(t, userRefresher.called)
	})

	t.Run("cluster ID doesn't match", func(t *testing.T) {
		clusterID := "c-955nj"
		oldTokenClusterName := token.ClusterName
		defer func() { token.ClusterName = oldTokenClusterName }()
		token.ClusterName = clusterID

		clusterReq := httptest.NewRequest(http.MethodGet, "/k8s/clusters/c-unknown/v1/management.cattle.io.authconfigs", nil)
		clusterReq.Header.Set("Authorization", "Bearer "+token.Name+":"+token.Token)

		userRefresher.reset()

		resp, err := authenticator.Authenticate(clusterReq)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("user doesn't exist", func(t *testing.T) {
		oldGetUserFunc := userLister.GetFunc
		defer func() { userLister.GetFunc = oldGetUserFunc }()
		userLister.GetFunc = func(namespace, name string) (*v3.User, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
		assert.False(t, userRefresher.called)
	})

	t.Run("user is disabled", func(t *testing.T) {
		oldGetUserFunc := userLister.GetFunc
		defer func() { userLister.GetFunc = oldGetUserFunc }()
		userLister.GetFunc = func(namespace, name string) (*v3.User, error) {
			return &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: userID,
				},
				Enabled: pointer.BoolPtr(false),
			}, nil
		}

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
		assert.False(t, userRefresher.called)
	})

	t.Run("error getting userattribute", func(t *testing.T) {
		oldGetUserAttributeFunc := userAttributeLister.GetFunc
		defer func() { userAttributeLister.GetFunc = oldGetUserAttributeFunc }()
		userAttributeLister.GetFunc = func(namespace, name string) (*v3.UserAttribute, error) {
			return nil, fmt.Errorf("some error")
		}

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
		assert.False(t, userRefresher.called)
	})

	t.Run("auth provider is disabled", func(t *testing.T) {
		oldIsDisabled := fakeProvider.disabled
		defer func() { fakeProvider.disabled = oldIsDisabled }()
		fakeProvider.disabled = true

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
		assert.False(t, userRefresher.called)
	})

	t.Run("auth provider doesn't exist", func(t *testing.T) {
		oldProvider := token.AuthProvider
		token.AuthProvider = "foo"
		defer func() { token.AuthProvider = oldProvider }()

		userRefresher.reset()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
		assert.False(t, userRefresher.called)
	})
}
