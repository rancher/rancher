package common

import (
	"errors"
	"strconv"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	"github.com/rancher/rancher/pkg/features"
	fake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			impClientMock := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
			userControllerMock.EXPECT().WithImpersonation(gomock.Any()).Return(impClientMock, nil).AnyTimes()
			impClientMock.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(user *v3.User) (*v3.User, error) {
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

func TestUserAttributeCreateOrUpdateWithSCIM(t *testing.T) {
	const (
		userID   = "u-abcdef"
		provider = "okta"
	)

	group := func(name string) v3.Principal {
		return v3.Principal{ObjectMeta: v1.ObjectMeta{Name: name}}
	}
	scimExtras := map[string][]string{
		UserAttributePrincipalID: {"okta_user://alice"},
		UserAttributeUserName:    {"alice"},
		"externalid":             {"00u123"},
		"email":                  {"alice@example.com"},
	}
	loginExtras := map[string][]string{
		UserAttributePrincipalID: {"okta_user://alice"},
		UserAttributeUserName:    {"alice.login"},
	}
	sameLoginExtras := map[string][]string{
		UserAttributePrincipalID: {"okta_user://alice"},
		UserAttributeUserName:    {"alice"},
	}
	stored := &v3.UserAttribute{
		ObjectMeta: v1.ObjectMeta{Name: userID},
		GroupPrincipals: map[string]v3.Principals{
			provider: {Items: []v3.Principal{group("okta_group://g1")}},
		},
		ExtraByProvider: map[string]map[string][]string{
			provider: scimExtras,
		},
	}
	loginTime := time.Now()

	tests := []struct {
		name        string
		scimEnabled bool
		stored      *v3.UserAttribute
		groups      []v3.Principal
		extras      map[string][]string
		loginTime   []time.Time
		wantWrite   string
		wantExtras  map[string][]string
		wantGroups  []string
	}{
		{
			name:        "SCIM enabled keeps stored keys login doesn't write",
			scimEnabled: true,
			stored:      stored,
			groups:      []v3.Principal{group("okta_group://g1")},
			extras:      loginExtras,
			loginTime:   []time.Time{loginTime},
			wantWrite:   "update",
			wantExtras: map[string][]string{
				UserAttributePrincipalID: {"okta_user://alice"},
				UserAttributeUserName:    {"alice.login"},
				"externalid":             {"00u123"},
				"email":                  {"alice@example.com"},
			},
			wantGroups: []string{"okta_group://g1"},
		},
		{
			name:       "SCIM not enabled replaces the extras",
			stored:     stored,
			groups:     []v3.Principal{group("okta_group://g1")},
			extras:     loginExtras,
			loginTime:  []time.Time{loginTime},
			wantWrite:  "update",
			wantExtras: loginExtras,
			wantGroups: []string{"okta_group://g1"},
		},
		{
			name:        "SCIM enabled replaces the groups",
			scimEnabled: true,
			stored:      stored,
			groups:      []v3.Principal{group("okta_group://g2")},
			extras:      sameLoginExtras,
			wantWrite:   "update",
			wantExtras:  scimExtras,
			wantGroups:  []string{"okta_group://g2"},
		},
		{
			name:        "SCIM enabled replaces the groups with an empty list",
			scimEnabled: true,
			stored:      stored,
			groups:      []v3.Principal{},
			extras:      sameLoginExtras,
			wantWrite:   "update",
			wantExtras:  scimExtras,
			wantGroups:  []string{},
		},
		{
			name:       "SCIM not enabled replaces the groups",
			stored:     stored,
			groups:     []v3.Principal{group("okta_group://g2")},
			extras:     sameLoginExtras,
			wantWrite:  "update",
			wantExtras: sameLoginExtras,
			wantGroups: []string{"okta_group://g2"},
		},
		{
			name:       "SCIM not enabled replaces the groups with an empty list",
			stored:     stored,
			groups:     []v3.Principal{},
			extras:     sameLoginExtras,
			wantWrite:  "update",
			wantExtras: sameLoginExtras,
			wantGroups: []string{},
		},
		{
			name:        "SCIM enabled makes no update when nothing changed",
			scimEnabled: true,
			stored:      stored,
			groups:      []v3.Principal{group("okta_group://g1")},
			extras:      sameLoginExtras,
		},
		{
			name:       "SCIM not enabled updates when login doesn't write every stored key",
			stored:     stored,
			groups:     []v3.Principal{group("okta_group://g1")},
			extras:     sameLoginExtras,
			wantWrite:  "update",
			wantExtras: sameLoginExtras,
			wantGroups: []string{"okta_group://g1"},
		},
		{
			name:        "SCIM enabled creates with the written extras",
			scimEnabled: true,
			groups:      []v3.Principal{group("okta_group://g1")},
			extras:      loginExtras,
			loginTime:   []time.Time{loginTime},
			wantWrite:   "create",
			wantExtras:  loginExtras,
			wantGroups:  []string{"okta_group://g1"},
		},
		{
			name:       "SCIM not enabled creates with the written extras",
			groups:     []v3.Principal{group("okta_group://g1")},
			extras:     loginExtras,
			loginTime:  []time.Time{loginTime},
			wantWrite:  "create",
			wantExtras: loginExtras,
			wantGroups: []string{"okta_group://g1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: the feature flag and the environment are global.
			t.Setenv("RANCHER_VERSION_TYPE", "prime")
			features.SCIM.Set(true)
			t.Cleanup(features.SCIM.Unset)

			ctrl := gomock.NewController(t)

			configMapCache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			configMapCache.EXPECT().Get(scimconfig.Namespace, "scim-config-"+provider).Return(&corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{Name: "scim-config-" + provider},
				Data:       map[string]string{"enabled": strconv.FormatBool(tt.scimEnabled)},
			}, nil).AnyTimes()

			userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
			userCache.EXPECT().Get(userID).Return(&v3.User{
				ObjectMeta: v1.ObjectMeta{Name: userID},
				Enabled:    ptr.To(true),
			}, nil).AnyTimes()

			var storedBefore *v3.UserAttribute
			userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
			userAttributes := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
			if tt.stored != nil {
				storedBefore = tt.stored.DeepCopy()
				userAttributeCache.EXPECT().Get(userID).Return(tt.stored, nil)
			} else {
				notFound := apierrors.NewNotFound(schema.GroupResource{}, userID)
				userAttributeCache.EXPECT().Get(userID).Return(nil, notFound)
				userAttributes.EXPECT().Get(userID, gomock.Any()).Return(nil, notFound)
			}

			var written *v3.UserAttribute
			var write string
			userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				write = "update"
				written = userAttribute.DeepCopy()
				return written, nil
			}).AnyTimes()
			userAttributes.EXPECT().Create(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				write = "create"
				written = userAttribute.DeepCopy()
				return written, nil
			}).AnyTimes()

			manager := userManager{
				userCache:          userCache,
				userAttributes:     userAttributes,
				userAttributeCache: userAttributeCache,
				configMapCache:     configMapCache,
			}

			err := manager.UserAttributeCreateOrUpdate(userID, provider, tt.groups, tt.extras, tt.loginTime...)
			require.NoError(t, err)

			// The cached object isn't changed.
			assert.Equal(t, storedBefore, tt.stored)

			require.Equal(t, tt.wantWrite, write)
			if tt.wantWrite == "" {
				return
			}
			assert.Equal(t, tt.wantExtras, written.ExtraByProvider[provider])
			var gotGroups []string
			for _, p := range written.GroupPrincipals[provider].Items {
				gotGroups = append(gotGroups, p.Name)
			}
			assert.ElementsMatch(t, tt.wantGroups, gotGroups)
		})
	}
}

func TestUserAttributeCreateOrUpdateNoGroups(t *testing.T) {
	const (
		userID   = "u-abcdef"
		provider = "okta"
	)

	group := func(name string) v3.Principal {
		return v3.Principal{ObjectMeta: v1.ObjectMeta{Name: name}}
	}
	scimExtras := map[string][]string{
		UserAttributePrincipalID: {"okta_user://alice"},
		UserAttributeUserName:    {"alice"},
		"externalid":             {"00u123"},
		"email":                  {"alice@example.com"},
	}
	loginExtras := map[string][]string{
		UserAttributePrincipalID: {"okta_user://alice"},
		UserAttributeUserName:    {"alice.login"},
	}
	sameLoginExtras := map[string][]string{
		UserAttributePrincipalID: {"okta_user://alice"},
		UserAttributeUserName:    {"alice"},
	}
	withGroups := &v3.UserAttribute{
		ObjectMeta: v1.ObjectMeta{Name: userID},
		GroupPrincipals: map[string]v3.Principals{
			provider: {Items: []v3.Principal{group("okta_group://g1")}},
		},
		ExtraByProvider: map[string]map[string][]string{
			provider: scimExtras,
		},
	}
	withoutGroupEntry := &v3.UserAttribute{
		ObjectMeta: v1.ObjectMeta{Name: userID},
		ExtraByProvider: map[string]map[string][]string{
			provider: scimExtras,
		},
	}
	loginTime := time.Now()

	tests := []struct {
		name        string
		scimEnabled bool
		stored      *v3.UserAttribute
		extras      map[string][]string
		loginTime   []time.Time
		wantWrite   string
		wantExtras  map[string][]string
		wantGroups  []string
	}{
		{
			name:        "SCIM enabled login keeps the stored groups",
			scimEnabled: true,
			stored:      withGroups,
			extras:      loginExtras,
			loginTime:   []time.Time{loginTime},
			wantWrite:   "update",
			wantExtras: map[string][]string{
				UserAttributePrincipalID: {"okta_user://alice"},
				UserAttributeUserName:    {"alice.login"},
				"externalid":             {"00u123"},
				"email":                  {"alice@example.com"},
			},
			wantGroups: []string{"okta_group://g1"},
		},
		{
			name:        "SCIM enabled update without a login time keeps the groups login stored",
			scimEnabled: true,
			stored:      withGroups,
			extras: map[string][]string{
				UserAttributePrincipalID: {"okta_user://alice"},
				UserAttributeUserName:    {"alice"},
				"externalid":             {"00u456"},
				"email":                  {""},
			},
			wantWrite: "update",
			wantExtras: map[string][]string{
				UserAttributePrincipalID: {"okta_user://alice"},
				UserAttributeUserName:    {"alice"},
				"externalid":             {"00u456"},
				"email":                  {""},
			},
			wantGroups: []string{"okta_group://g1"},
		},
		{
			name:        "SCIM enabled writes an empty group entry when the provider has none",
			scimEnabled: true,
			stored:      withoutGroupEntry,
			extras:      scimExtras,
			wantWrite:   "update",
			wantExtras:  scimExtras,
			wantGroups:  []string{},
		},
		{
			name:        "SCIM enabled makes no update when nothing changed",
			scimEnabled: true,
			stored:      withGroups,
			extras:      sameLoginExtras,
		},
		{
			name:        "SCIM enabled creates with an empty group entry",
			scimEnabled: true,
			extras:      scimExtras,
			wantWrite:   "create",
			wantExtras:  scimExtras,
			wantGroups:  []string{},
		},
		{
			name:       "SCIM not enabled replaces the groups with an empty list",
			stored:     withGroups,
			extras:     loginExtras,
			loginTime:  []time.Time{loginTime},
			wantWrite:  "update",
			wantExtras: loginExtras,
			wantGroups: []string{},
		},
		{
			name:       "SCIM not enabled creates with an empty group entry",
			extras:     loginExtras,
			loginTime:  []time.Time{loginTime},
			wantWrite:  "create",
			wantExtras: loginExtras,
			wantGroups: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: the feature flag and the environment are global.
			t.Setenv("RANCHER_VERSION_TYPE", "prime")
			features.SCIM.Set(true)
			t.Cleanup(features.SCIM.Unset)

			ctrl := gomock.NewController(t)

			configMapCache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			configMapCache.EXPECT().Get(scimconfig.Namespace, "scim-config-"+provider).Return(&corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{Name: "scim-config-" + provider},
				Data:       map[string]string{"enabled": strconv.FormatBool(tt.scimEnabled)},
			}, nil).AnyTimes()

			userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
			userCache.EXPECT().Get(userID).Return(&v3.User{
				ObjectMeta: v1.ObjectMeta{Name: userID},
				Enabled:    ptr.To(true),
			}, nil).AnyTimes()

			var storedBefore *v3.UserAttribute
			userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
			userAttributes := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
			if tt.stored != nil {
				storedBefore = tt.stored.DeepCopy()
				userAttributeCache.EXPECT().Get(userID).Return(tt.stored, nil)
			} else {
				notFound := apierrors.NewNotFound(schema.GroupResource{}, userID)
				userAttributeCache.EXPECT().Get(userID).Return(nil, notFound)
				userAttributes.EXPECT().Get(userID, gomock.Any()).Return(nil, notFound)
			}

			var written *v3.UserAttribute
			var write string
			userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				write = "update"
				written = userAttribute.DeepCopy()
				return written, nil
			}).AnyTimes()
			userAttributes.EXPECT().Create(gomock.Any()).DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				write = "create"
				written = userAttribute.DeepCopy()
				return written, nil
			}).AnyTimes()

			manager := userManager{
				userCache:          userCache,
				userAttributes:     userAttributes,
				userAttributeCache: userAttributeCache,
				configMapCache:     configMapCache,
			}

			err := manager.UserAttributeCreateOrUpdateNoGroups(userID, provider, tt.extras, tt.loginTime...)
			require.NoError(t, err)

			// The cached object isn't changed.
			assert.Equal(t, storedBefore, tt.stored)

			require.Equal(t, tt.wantWrite, write)
			if tt.wantWrite == "" {
				return
			}
			assert.Equal(t, tt.wantExtras, written.ExtraByProvider[provider])
			require.Contains(t, written.GroupPrincipals, provider)
			var gotGroups []string
			for _, p := range written.GroupPrincipals[provider].Items {
				gotGroups = append(gotGroups, p.Name)
			}
			assert.ElementsMatch(t, tt.wantGroups, gotGroups)
		})
	}
}
