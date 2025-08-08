package common

import (
	"errors"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	fake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

func TestSetPrincipalOnCurrentUserByUserID(t *testing.T) {
	testCases := []struct {
		name             string
		userID           string
		principal        v3.Principal
		existingUser     *v3.User
		existingError    error
		principalUser    *v3.User
		principalError   error
		expectedUser     *v3.User
		expectedError    error
		expectedToUpdate bool
	}{
		{
			name:   "successfully add principal to user",
			userID: "user1",
			principal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "github_user1",
				},
				DisplayName: "github_user1",
				Provider:    "github",
			},
			existingUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user"},
			},
			principalUser:  nil,
			principalError: nil,
			expectedUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user", "github_user1"},
			},
			expectedError:    nil,
			expectedToUpdate: true,
		},
		{
			name:   "user retrieval fails",
			userID: "user1",
			principal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
				},
				Provider: "github",
			},
			existingUser:     nil,
			existingError:    errors.New("user not found"),
			expectedError:    errors.New("user not found"),
			expectedToUpdate: false,
		},
		{
			name:   "principal conflict with another user",
			userID: "user1",
			principal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "github_user1",
				},
				DisplayName: "github_user1",
				Provider:    "github",
			},
			existingUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user"},
			},
			principalUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user2",
					UID:  "uid2",
				},
				PrincipalIDs: []string{"local://user", "github_user1"},
			},
			principalError: nil,
			expectedUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user"},
			},
			expectedError:    errors.New("refusing to set principal on user that is already bound to another user"),
			expectedToUpdate: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			userControllerMock := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			mockUserIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			indexers := map[string]cache.IndexFunc{
				userByPrincipalIndex: userByPrincipal,
			}
			mockUserIndexer.AddIndexers(indexers)

			userControllerMock.EXPECT().Get(test.userID, gomock.Any()).Return(test.existingUser, test.existingError)
			if test.principalUser != nil {
				userControllerMock.EXPECT().List(gomock.Any()).Return(&v3.UserList{Items: []v3.User{*test.principalUser}}, test.principalError).AnyTimes()
			} else {
				userControllerMock.EXPECT().List(gomock.Any()).Return(&v3.UserList{}, test.principalError).AnyTimes()
			}
			userControllerMock.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(user *v3.User) (*v3.User, error) {
				u := user.DeepCopy()
				return u, nil
			})

			um := &userManager{
				users:       userControllerMock,
				userIndexer: mockUserIndexer,
			}

			result, err := um.SetPrincipalOnCurrentUserByUserID(test.userID, test.principal)

			if test.expectedError != nil {
				assert.EqualError(t, err, test.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedUser, result)
			}
		})
	}
}

func TestCheckAccess(t *testing.T) {
	testCases := []struct {
		name                string
		accessMode          string
		allowedPrincipalIDs []string
		userPrincipalID     string
		groups              []v3.Principal
		user                *v3.User
		userErr             error
		expectedResult      bool
		expectedError       error
	}{
		{
			name:                "Unrestricted access, should allow",
			accessMode:          "unrestricted",
			allowedPrincipalIDs: []string{},
			userPrincipalID:     "local://user",
			expectedResult:      true,
			expectedError:       nil,
		},
		{
			name:                "Required access, principal allowed",
			accessMode:          "required",
			allowedPrincipalIDs: []string{"local://user", "github://user1"},
			userPrincipalID:     "local://user",
			user: &v3.User{
				PrincipalIDs: []string{"local://user", "github://user1"},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name:                "Restricted access, no matching principal",
			accessMode:          "restricted",
			allowedPrincipalIDs: []string{"github://user2"},
			userPrincipalID:     "local://user",
			user: &v3.User{
				PrincipalIDs: []string{"local://user"},
			},
			groups: []v3.Principal{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "github://group1",
					},
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name:                "Unsupported accessMode",
			accessMode:          "unknown",
			allowedPrincipalIDs: []string{"local://user"},
			userPrincipalID:     "local://user",
			expectedResult:      false,
			expectedError:       errors.New("Unsupported accessMode: unknown"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mockUserIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			indexers := map[string]cache.IndexFunc{
				userByPrincipalIndex: userByPrincipal,
			}
			mockUserIndexer.AddIndexers(indexers)

			userControllerMock := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			um := &userManager{
				users:       userControllerMock,
				userIndexer: mockUserIndexer,
			}

			userControllerMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options v1.GetOptions) (*v3.User, error) {
				return test.user, test.userErr
			}).AnyTimes()

			result, err := um.CheckAccess(test.accessMode, test.allowedPrincipalIDs, test.userPrincipalID, test.groups)

			if test.expectedError != nil {
				assert.EqualError(t, err, test.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}

func TestUserAttributeCreateOrUpdateSetsLastLoginTime(t *testing.T) {
	createdUserAttribute := &v3.UserAttribute{}
	userID := "u-abcdef"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).Return(&v3.User{
		ObjectMeta: v1.ObjectMeta{
			Name: userID,
		},
		Enabled: ptr.To(true),
	}, nil,
	).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).Return(&v3.UserAttribute{}, nil).AnyTimes()

	userAttributes := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		return userAttribute.DeepCopy(), nil
	}).AnyTimes()
	userAttributes.EXPECT().Create(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		createdUserAttribute = userAttribute.DeepCopy()
		return createdUserAttribute, nil
	}).AnyTimes()

	manager := userManager{
		userCache:          userCache,
		userAttributes:     userAttributes,
		userAttributeCache: userAttributeCache,
	}

	groupPrincipals := []v3.Principal{}
	userExtraInfo := map[string][]string{}

	loginTime := time.Now()
	err := manager.UserAttributeCreateOrUpdate(userID, "provider", groupPrincipals, userExtraInfo, loginTime)
	assert.NoError(t, err)

	// Make sure login time is set and truncated to seconds.
	assert.Equal(t, loginTime.Truncate(time.Second), createdUserAttribute.LastLogin.Time)
}

func TestUserAttributeCreateOrUpdateUpdatesGroups(t *testing.T) {
	updatedUserAttribute := &v3.UserAttribute{}
	userID := "u-abcdef"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).Return(&v3.User{
		ObjectMeta: v1.ObjectMeta{
			Name: userID,
		},
		Enabled: ptr.To(true),
	}, nil,
	).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).Return(&v3.UserAttribute{
		ObjectMeta: v1.ObjectMeta{
			Name: userID,
		},
	}, nil).AnyTimes()

	userAttributes := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		updatedUserAttribute = userAttribute.DeepCopy()
		return updatedUserAttribute, nil
	}).AnyTimes()
	userAttributes.EXPECT().Create(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		return userAttribute.DeepCopy(), nil
	}).AnyTimes()

	manager := userManager{
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
		userAttributes:     userAttributes,
	}

	groupPrincipals := []v3.Principal{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "group1",
			},
		},
	}
	userExtraInfo := map[string][]string{}

	err := manager.UserAttributeCreateOrUpdate(userID, "provider", groupPrincipals, userExtraInfo)
	assert.NoError(t, err)

	require.Len(t, updatedUserAttribute.GroupPrincipals, 1)
	principals := updatedUserAttribute.GroupPrincipals["provider"]
	require.NotEmpty(t, principals)
	require.Len(t, principals.Items, 1)
	assert.Equal(t, principals.Items[0].Name, "group1")
}
