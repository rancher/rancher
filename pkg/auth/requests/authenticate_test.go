package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
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

type fakeUserAuthRefresherArgs struct {
	userID string
	force  bool
}
type fakeUserAuthRefresher struct {
	calledTimes atomic.Int32
	done        chan fakeUserAuthRefresherArgs
}

func (*fakeUserAuthRefresher) TriggerAllUserRefresh() {}
func (r *fakeUserAuthRefresher) TriggerUserRefresh(userID string, force bool) {
	r.calledTimes.Add(1)
	if r.done != nil {
		r.done <- fakeUserAuthRefresherArgs{
			userID: userID,
			force:  force,
		}
	}
}

type fakeProvider struct {
	Name                   string
	IsDisabledProviderFunc func() (bool, error)
}

func (p *fakeProvider) IsDisabledProvider() (bool, error) {
	if p.IsDisabledProviderFunc != nil {
		return p.IsDisabledProviderFunc()
	}
	return false, nil
}

func (p *fakeProvider) GetName() string {
	return p.Name
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
		Name: "fake",
	}

	providers.Providers = map[string]common.AuthProvider{
		fakeProvider.Name: fakeProvider,
	}

	now := time.Now()
	userID := "u-abcdef"
	token := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "token-v2rcx",
			CreationTimestamp: metav1.NewTime(now),
		},
		Token:        "jnb9tksmnctvgbn92ngbkptblcjwg4pmfp98wqj29wk5kv85ktg59s",
		AuthProvider: fakeProvider.Name,
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
					fakeProvider.Name: {
						Items: []apiv3.Principal{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: fakeProvider.Name + "_group://56789",
								},
								MemberOf:      true,
								LoginName:     "rancher",
								DisplayName:   "rancher",
								PrincipalType: "group",
								Provider:      fakeProvider.Name,
							},
						},
					},
				},
				ExtraByProvider: map[string]map[string][]string{
					fakeProvider.Name: {
						common.UserAttributePrincipalID: {fakeProvider.Name + "_user://12345"},
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

	refresher := &fakeUserAuthRefresher{}

	authenticator := tokenAuthenticator{
		ctx:                 context.Background(),
		tokenIndexer:        mockIndexer,
		userAttributeLister: userAttributeLister,
		userLister:          userLister,
		userAuthRefresher:   refresher,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/namespaces", nil)
	req.Header.Set("Authorization", "Bearer "+token.Name+":"+token.Token)

	t.Run("authenticated", func(t *testing.T) {
		refresher.done = make(chan fakeUserAuthRefresherArgs)
		refresher.calledTimes.Store(0)
		defer func() {
			refresher.done = nil
		}()

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		refresherArgs := <-refresher.done // Wait for the provider refresh to finish.
		assert.Equal(t, int32(1), refresher.calledTimes.Load())

		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
		assert.Equal(t, userID, resp.User)
		assert.Equal(t, userID, resp.UserPrincipal)
		assert.Equal(t, userID, refresherArgs.userID)
		assert.False(t, refresherArgs.force)
		assert.Contains(t, resp.Groups, fakeProvider.Name+"_group://56789")
		assert.Contains(t, resp.Groups, "system:cattle:authenticated")
		assert.Contains(t, resp.Extras[common.UserAttributePrincipalID], fakeProvider.Name+"_user://12345")
		assert.Contains(t, resp.Extras[common.UserAttributeUserName], "fake-user")
	})

	t.Run("authenticated if userattribute doesn't exist", func(t *testing.T) {
		oldGetUserAttributeFunc := userAttributeLister.GetFunc
		defer func() { userAttributeLister.GetFunc = oldGetUserAttributeFunc }()
		userAttributeLister.GetFunc = func(namespace, name string) (*v3.UserAttribute, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
	})

	t.Run("provider refresh is not called for system users", func(t *testing.T) {
		refresher.calledTimes.Store(0)

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

		resp, err := authenticator.Authenticate(req)
		require.NoError(t, err)
		assert.Equal(t, int32(0), refresher.calledTimes.Load())

		require.NotNil(t, resp)
		assert.True(t, resp.IsAuthed)
	})

	t.Run("token disabled", func(t *testing.T) {
		oldTokenEnabled := token.Enabled
		defer func() { token.Enabled = oldTokenEnabled }()
		token.Enabled = pointer.BoolPtr(false)

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("user doesn't exist", func(t *testing.T) {
		oldGetUserFunc := userLister.GetFunc
		defer func() { userLister.GetFunc = oldGetUserFunc }()
		userLister.GetFunc = func(namespace, name string) (*v3.User, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("user disabled", func(t *testing.T) {
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

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("error getting userattribute", func(t *testing.T) {
		oldGetUserAttributeFunc := userAttributeLister.GetFunc
		defer func() { userAttributeLister.GetFunc = oldGetUserAttributeFunc }()
		userAttributeLister.GetFunc = func(namespace, name string) (*v3.UserAttribute, error) {
			return nil, fmt.Errorf("some error")
		}

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("auth provider disabled", func(t *testing.T) {
		oldIsDisabledProviderFunc := fakeProvider.IsDisabledProviderFunc
		defer func() { fakeProvider.IsDisabledProviderFunc = oldIsDisabledProviderFunc }()
		fakeProvider.IsDisabledProviderFunc = func() (bool, error) { return true, nil }

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("auth provider doesn't exist", func(t *testing.T) {
		oldProvider := token.AuthProvider
		token.AuthProvider = "foo"
		defer func() { token.AuthProvider = oldProvider }()

		resp, err := authenticator.Authenticate(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})
}
