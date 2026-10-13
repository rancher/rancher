package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	"github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var testDefaultGetConfig = func(string) scimconfig.Config { return scimconfig.Default() }

func TestGetRancherGroupMembers(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"
	groupName := "Engineering"

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
		{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-yypnjwjmkq"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-e2hb6ijutz"}}, // No group membership.
		{ObjectMeta: metav1.ObjectMeta{Name: "u-hl3yygin2t"}}, // No user attribute.
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-b4qkhsnliz",
			},
			PrincipalIDs: []string{"system://local"}, // System account for local cluster.
		},
	}, nil)

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		switch name {
		case "u-mo773yttt4":
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
				GroupPrincipals: map[string]v3.Principals{
					provider: {
						Items: []v3.Principal{
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, groupName)}, DisplayName: groupName},
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Other Group")}, DisplayName: "Other Group"},
						},
					},
				},
				ExtraByProvider: map[string]map[string][]string{
					provider: {"username": {"john.doe"}},
				},
			}, nil
		case "u-yypnjwjmkq":
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "u-yypnjwjmkq"},
				GroupPrincipals: map[string]v3.Principals{
					provider: {
						Items: []v3.Principal{
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Different Group")}, DisplayName: "Different Group"},
						},
					},
				},
				ExtraByProvider: map[string]map[string][]string{
					provider: {"username": {"jane.smith"}},
				},
			}, nil
		case "u-e2hb6ijutz": // Missing group membership.
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "u-e2hb6ijutz"},
				ExtraByProvider: map[string]map[string][]string{
					provider: {"username": {"alice.wonder"}},
				},
			}, nil
		default:
			return nil, apierrors.NewNotFound(v3.Resource("userattributes"), name)
		}
	}).AnyTimes()

	srv := &SCIMServer{
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
		getConfig:          testDefaultGetConfig,
	}

	members, err := srv.getRancherGroupMembers(provider, groupPrincipalName(provider, groupName))

	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "u-mo773yttt4", members[0].Value)
	assert.Equal(t, "john.doe", members[0].Display)
	assert.Equal(t, userResource, members[0].Type)
}

func TestGetAllRancherGroupMembers(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
		{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-yypnjwjmkq"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-e2hb6ijutz"}}, // No group membership.
		{ObjectMeta: metav1.ObjectMeta{Name: "u-hl3yygin2t"}}, // No user attribute.
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-b4qkhsnliz",
			},
			PrincipalIDs: []string{"system://local"}, // System account - should be skipped.
		},
	}, nil)

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		switch name {
		case "u-mo773yttt4":
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
				GroupPrincipals: map[string]v3.Principals{
					provider: {
						Items: []v3.Principal{
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"},
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Architects")}, DisplayName: "Architects"},
						},
					},
				},
				ExtraByProvider: map[string]map[string][]string{
					provider: {"username": {"john.doe"}},
				},
			}, nil
		case "u-yypnjwjmkq":
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "u-yypnjwjmkq"},
				GroupPrincipals: map[string]v3.Principals{
					provider: {
						Items: []v3.Principal{
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"},
							{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Developers")}, DisplayName: "Developers"},
						},
					},
				},
				ExtraByProvider: map[string]map[string][]string{
					provider: {"username": {"jane.smith"}},
				},
			}, nil
		case "u-e2hb6ijutz": // Missing group membership.
			return &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{Name: "u-e2hb6ijutz"},
				ExtraByProvider: map[string]map[string][]string{
					provider: {"username": {"alice.wonder"}},
				},
			}, nil
		default:
			return nil, apierrors.NewNotFound(v3.Resource("userattributes"), name)
		}
	}).AnyTimes()

	srv := &SCIMServer{
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
		getConfig:          testDefaultGetConfig,
	}

	groups, err := srv.getAllRancherGroupMembers(provider)

	require.NoError(t, err)
	require.Len(t, groups, 3) // Engineering, Architects, Developers.

	// Verify Engineering group has 2 members.
	engineers := groups[groupPrincipalName(provider, "Engineering")]
	require.Len(t, engineers, 2)

	// Check both members are present (order may vary).
	memberNames := []string{engineers[0].Value, engineers[1].Value}
	assert.Contains(t, memberNames, "u-mo773yttt4")
	assert.Contains(t, memberNames, "u-yypnjwjmkq")

	// Verify Architects group has 1 member.
	architects := groups[groupPrincipalName(provider, "Architects")]
	require.Len(t, architects, 1)
	assert.Equal(t, "u-mo773yttt4", architects[0].Value)
	assert.Equal(t, "john.doe", architects[0].Display)
	assert.Equal(t, userResource, architects[0].Type)

	// Verify Developers group has 1 member.
	developers := groups[groupPrincipalName(provider, "Developers")]
	require.Len(t, developers, 1)
	assert.Equal(t, "u-yypnjwjmkq", developers[0].Value)
	assert.Equal(t, "jane.smith", developers[0].Display)
	assert.Equal(t, userResource, developers[0].Type)
}

func TestResolveGroupMembers(t *testing.T) {
	provider := "okta"
	groupName := "Engineering"

	t.Run("rejects nested group member", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// getRancherGroupMembers: no current members.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			userCache: userCache,
			getConfig: testDefaultGetConfig,
		}

		members, err := srv.resolveGroupMembers(provider, groupPrincipalName(provider, groupName), []scimMember{
			{Value: "grp-xyz", Type: "Group", Display: "SubTeam"},
		})

		require.Error(t, err)
		assert.Nil(t, members)
		scimErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, scimErr.Status)
	})

	t.Run("rejects unsupported member type", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// getRancherGroupMembers: no current members.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			userCache: userCache,
			getConfig: testDefaultGetConfig,
		}

		members, err := srv.resolveGroupMembers(provider, groupPrincipalName(provider, groupName), []scimMember{
			{Value: "something-else", Type: "SomethingElse", Display: "Something Else"},
		})

		require.Error(t, err)
		assert.Nil(t, members)
		scimErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, scimErr.Status)
	})

	t.Run("rejects member not found in pre-flight", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// getRancherGroupMembers: no current members.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		// User does not exist.
		notFound := &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Reason: metav1.StatusReasonNotFound,
				Code:   http.StatusNotFound,
			},
		}
		userCache.EXPECT().Get("u-missing").Return(nil, notFound)

		srv := &SCIMServer{
			userCache: userCache,
			getConfig: testDefaultGetConfig,
		}

		members, err := srv.resolveGroupMembers(provider, groupPrincipalName(provider, groupName), []scimMember{
			{Value: "u-missing", Display: "missing.user"},
		})

		require.Error(t, err)
		assert.Nil(t, members)
		scimErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, scimErr.Status)
	})
}

func TestSyncGroupMembers(t *testing.T) {
	provider := "okta"
	groupName := "Engineering"

	t.Run("adds new members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)

		// Adding one member.
		newMember := scimMember{Value: "u-mo773yttt4", Display: "john.doe"}
		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{}},
			},
		}
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-mo773yttt4").Return(existingAttr, false, nil)

		updatedAttr := existingAttr.DeepCopy()
		updatedAttr.GroupPrincipals[provider] = v3.Principals{
			Items: []v3.Principal{{
				ObjectMeta:    metav1.ObjectMeta{Name: fmt.Sprintf("%s_group://%s", provider, groupName)},
				DisplayName:   groupName,
				MemberOf:      true,
				PrincipalType: "group",
				Provider:      provider,
			}},
		}

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(updatedAttr, nil)

		srv := &SCIMServer{
			userCache:      userCache,
			userMGR:        userMGR,
			userAttributes: userAttrClient,
			getConfig:      testDefaultGetConfig,
		}

		err := srv.syncGroupMembers(provider, groupPrincipalName(provider, groupName), groupName, []scimMember{newMember}, map[string]string{})
		require.NoError(t, err)
	})

	t.Run("adds member that has no attribute yet", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		newMember := scimMember{Value: "u-new", Display: "new.user"}
		user := &v3.User{
			TypeMeta:   metav1.TypeMeta{APIVersion: "management.cattle.io/v3", Kind: "User"},
			ObjectMeta: metav1.ObjectMeta{Name: "u-new", UID: "uid-new"},
		}

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get("u-new").Return(user, nil).AnyTimes()

		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-new").Return(&v3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "u-new"},
			GroupPrincipals: map[string]v3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}, true, nil)

		var created *v3.UserAttribute
		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *v3.UserAttribute) (*v3.UserAttribute, error) {
			created = a
			return a, nil
		})

		srv := &SCIMServer{
			userCache:      userCache,
			userMGR:        userMGR,
			userAttributes: userAttrClient,
			getConfig:      testDefaultGetConfig,
		}

		err := srv.syncGroupMembers(provider, groupPrincipalName(provider, groupName), groupName, []scimMember{newMember}, map[string]string{})
		require.NoError(t, err)

		require.NotNil(t, created)
		items := created.GroupPrincipals[provider].Items
		require.Len(t, items, 1)
		assert.Equal(t, groupPrincipalName(provider, groupName), items[0].Name)
	})

	t.Run("removes members not in new list", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// Current state: one member.
		existingUser := &v3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}}

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {
					Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, groupName)}, DisplayName: groupName}},
				},
			},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"john.doe"}},
			},
		}

		// Remove the member.
		userCache.EXPECT().Get("u-mo773yttt4").Return(existingUser, nil)
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-mo773yttt4").Return(existingAttr, false, nil)

		updatedAttr := existingAttr.DeepCopy()
		updatedAttr.GroupPrincipals[provider] = v3.Principals{Items: []v3.Principal{}}

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(updatedAttr, nil)

		srv := &SCIMServer{
			userCache:      userCache,
			userMGR:        userMGR,
			userAttributes: userAttrClient,
			getConfig:      testDefaultGetConfig,
		}

		err := srv.syncGroupMembers(provider, groupPrincipalName(provider, groupName), groupName, []scimMember{}, map[string]string{"u-mo773yttt4": "john.doe"})
		require.NoError(t, err)
	})
}

func TestUpdateGroupMemberDisplayName(t *testing.T) {
	t.Parallel()

	provider := "okta"
	principalName := groupPrincipalName(provider, "Engineering")

	t.Run("updates stale displayName", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)

		memberID := "u-mo773yttt4"
		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: memberID},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{
					{ObjectMeta: metav1.ObjectMeta{Name: principalName}, DisplayName: "Old Name"},
				}},
			},
		}

		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute(memberID).Return(existingAttr, false, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			items := attr.GroupPrincipals[provider].Items
			require.Len(t, items, 1)
			assert.Equal(t, "Engineering", items[0].DisplayName)
			return attr, nil
		})

		srv := &SCIMServer{
			userMGR:        userMGR,
			userAttributes: userAttrClient,
		}

		err := srv.updateGroupMemberDisplayName(provider, principalName, "Engineering", memberID)
		require.NoError(t, err)
	})

	t.Run("no update when displayName is current", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)

		memberID := "u-mo773yttt4"
		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: memberID},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{
					{ObjectMeta: metav1.ObjectMeta{Name: principalName}, DisplayName: "Engineering"},
				}},
			},
		}

		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute(memberID).Return(existingAttr, false, nil)

		// No Update call expected.
		srv := &SCIMServer{
			userMGR: userMGR,
		}

		err := srv.updateGroupMemberDisplayName(provider, principalName, "Engineering", memberID)
		require.NoError(t, err)
	})
}

func TestSyncGroupMembersUpdatesStaleDisplayName(t *testing.T) {
	ctrl := gomock.NewController(t)

	provider := "okta"
	groupName := "Engineering"
	principalName := groupPrincipalName(provider, groupName)
	memberID := "u-mo773yttt4"

	// User is already a member but with a stale DisplayName.
	existingAttr := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: memberID},
		GroupPrincipals: map[string]v3.Principals{
			provider: {Items: []v3.Principal{
				{ObjectMeta: metav1.ObjectMeta{Name: principalName}, DisplayName: "Old Name"},
			}},
		},
		ExtraByProvider: map[string]map[string][]string{
			provider: {"username": {"john.doe"}},
		},
	}

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(memberID).Return(&v3.User{
		ObjectMeta: metav1.ObjectMeta{Name: memberID},
	}, nil).AnyTimes()

	userMGR := mocks.NewMockManager(ctrl)
	userMGR.EXPECT().EnsureAndGetUserAttribute(memberID).Return(existingAttr, false, nil)

	userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttrClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
		items := attr.GroupPrincipals[provider].Items
		require.Len(t, items, 1)
		assert.Equal(t, "Engineering", items[0].DisplayName)
		return attr, nil
	})

	srv := &SCIMServer{
		userCache:      userCache,
		userAttributes: userAttrClient,
		userMGR:        userMGR,
		getConfig:      testDefaultGetConfig,
	}

	err := srv.syncGroupMembers(provider, principalName, groupName, []scimMember{
		{Value: memberID, Display: "john.doe"},
	}, map[string]string{memberID: "john.doe"})
	require.NoError(t, err)
}

func TestApplyPatchGroup(t *testing.T) {
	cfg := scimconfig.Default()
	externalIDCfg := scimconfig.Config{GroupIDAttribute: scimconfig.GroupIDExternalID}

	t.Run("replaces externalId", func(t *testing.T) {
		group := &v3.Group{ExternalID: "old-id"}
		op := patchOp{Op: "replace", Path: "externalId", Value: "new-id"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("no update when value same", func(t *testing.T) {
		group := &v3.Group{ExternalID: "same-id"}
		op := patchOp{Op: "replace", Path: "externalId", Value: "same-id"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("bulk replace", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old", ExternalID: "old-id"}
		op := patchOp{
			Op:   "replace",
			Path: "",
			Value: map[string]any{
				"externalId": "new-id",
			},
		}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("rejects unsupported path", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{Op: "replace", Path: "unsupported", Value: "value"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
	})

	t.Run("replaces displayName when externalId is group ID", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old Name", ExternalID: "ext-123"}
		op := patchOp{Op: "replace", Path: "displayName", Value: "New Name"}

		updated, err := applyPatchGroup(group, op, externalIDCfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "New Name", group.DisplayName)
	})

	t.Run("rejects displayName change when displayName is group ID", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old Name"}
		op := patchOp{Op: "replace", Path: "displayName", Value: "New Name"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
		assert.ErrorContains(t, err, "cannot be changed")
	})

	t.Run("allows displayName no-op when displayName is group ID", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Same Name"}
		op := patchOp{Op: "replace", Path: "displayName", Value: "Same Name"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("rejects externalId change when externalId is group ID", func(t *testing.T) {
		group := &v3.Group{ExternalID: "old-id"}
		op := patchOp{Op: "replace", Path: "externalId", Value: "new-id"}

		updated, err := applyPatchGroup(group, op, externalIDCfg)

		require.Error(t, err)
		assert.False(t, updated)
		assert.ErrorContains(t, err, "cannot be changed")
	})

	t.Run("allows externalId no-op when externalId is group ID", func(t *testing.T) {
		group := &v3.Group{ExternalID: "same-id"}
		op := patchOp{Op: "add", Path: "externalId", Value: "same-id"}

		updated, err := applyPatchGroup(group, op, externalIDCfg)

		require.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("rejects invalid displayName value type", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{Op: "replace", Path: "displayName", Value: 123}

		updated, err := applyPatchGroup(group, op, externalIDCfg)

		require.Error(t, err)
		assert.False(t, updated)
	})

	t.Run("URN-prefixed externalId", func(t *testing.T) {
		group := &v3.Group{ExternalID: "old-id"}
		op := patchOp{Op: "replace", Path: "urn:ietf:params:scim:schemas:core:2.0:Group:externalId", Value: "new-id"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("URN-prefixed wrong resource type", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{Op: "replace", Path: "urn:ietf:params:scim:schemas:core:2.0:User:userName", Value: "test"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
		assert.ErrorContains(t, err, "does not match")
	})

	t.Run("add externalId", func(t *testing.T) {
		group := &v3.Group{ExternalID: "old-id"}
		op := patchOp{Op: "add", Path: "externalId", Value: "new-id"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("add externalId no-op when value same", func(t *testing.T) {
		group := &v3.Group{ExternalID: "same-id"}
		op := patchOp{Op: "add", Path: "externalId", Value: "same-id"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("add displayName when externalId is group ID", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old Name", ExternalID: "ext-123"}
		op := patchOp{Op: "add", Path: "displayName", Value: "New Name"}

		updated, err := applyPatchGroup(group, op, externalIDCfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "New Name", group.DisplayName)
	})

	t.Run("add rejects displayName change when displayName is group ID", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old Name"}
		op := patchOp{Op: "add", Path: "displayName", Value: "New Name"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
		assert.ErrorContains(t, err, "cannot be changed")
	})

	t.Run("add bulk", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old", ExternalID: "old-id"}
		op := patchOp{
			Op:   "add",
			Path: "",
			Value: map[string]any{
				"externalId": "new-id",
			},
		}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("URN-prefixed add externalId", func(t *testing.T) {
		group := &v3.Group{ExternalID: "old-id"}
		op := patchOp{Op: "add", Path: "urn:ietf:params:scim:schemas:core:2.0:Group:externalId", Value: "new-id"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("add rejects unsupported path", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{Op: "add", Path: "unsupported", Value: "value"}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
	})

	t.Run("rejects empty attribute name in bulk", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{
			Op:   "add",
			Path: "",
			Value: map[string]any{
				"": "value",
			},
		}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
		var scimErr *Error
		require.ErrorAs(t, err, &scimErr)
		assert.Equal(t, http.StatusBadRequest, scimErr.Status)
	})

	t.Run("bulk wraps inner SCIM error so callers can extract scimType", func(t *testing.T) {
		group := &v3.Group{DisplayName: "Old Name"}
		op := patchOp{
			Op:   "add",
			Path: "",
			Value: map[string]any{
				"displayName": "New Name",
			},
		}

		updated, err := applyPatchGroup(group, op, cfg)

		require.Error(t, err)
		assert.False(t, updated)
		var scimErr *Error
		require.ErrorAs(t, err, &scimErr)
		assert.Equal(t, "mutability", scimErr.ScimType)
	})
}

func TestExtractMemberValueFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "valid path with value eq",
			path:     `members[value eq "u-mo773yttt4"]`,
			expected: "u-mo773yttt4",
		},
		{
			name:     "valid path with spaces",
			path:     `members[value eq "user abc"]`,
			expected: "user abc",
		},
		{
			name:     "no quotes",
			path:     `members[value eq u-mo773yttt4]`,
			expected: "",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "only opening quote",
			path:     `members[value eq "u-mo773yttt4`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMemberValueFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatchGroup(t *testing.T) {
	provider := "okta"
	groupID := "grp-abc123"

	t.Run("add members operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// Adding member.
		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-mo773yttt4"},
			PrincipalIDs: []string{"local://u-mo773yttt4", provider + "_user://john.doe"},
		}, nil).Times(2) // Once in resolveGroupMembers, once inside addGroupMember.
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{provider: {Items: []v3.Principal{}}},
		}, false, nil)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(&v3.UserAttribute{}, nil)

		// First call in resolveGroupMembers: the group has no members yet.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)
		// Second call for the final getRancherGroupMembers.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}},
		}, nil)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"}}},
			},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"john.doe"}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
			userAttributes:     userAttrClient,
			getConfig:          testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "add",
					"path": "members",
					"value": []map[string]any{
						{"value": "u-mo773yttt4", "display": "john.doe"},
					},
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Engineering", resp["displayName"])

		// Members should be present in response.
		if members, ok := resp["members"].([]any); ok {
			// Members list is returned (could be empty based on mocks).
			assert.NotNil(t, members)
		}

		meta, ok := resp["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, groupResource, meta["resourceType"])

		wantLocation := "/v1-scim/" + provider + "/Groups/" + groupID
		assert.Contains(t, meta["location"], wantLocation)
		assert.Contains(t, w.Header().Get("Location"), wantLocation)

	})

	t.Run("add group as member is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		srv := &SCIMServer{
			groupsCache: groupCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "add",
					"path": "members",
					"value": []map[string]any{
						{"value": "grp-xyz", "display": "SubTeam", "type": "Group"},
					},
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
	})

	t.Run("add unsupported member type is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		srv := &SCIMServer{
			groupsCache: groupCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "add",
					"path": "members",
					"value": []map[string]any{
						{"value": "something-else", "display": "Something Else", "type": "SomethingElse"},
					},
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var respBad Error
		err := json.Unmarshal(w.Body.Bytes(), &respBad)
		require.NoError(t, err)
		assert.Contains(t, respBad.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, respBad.Status)
	})

	t.Run("add member that does not exist returns 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// resolveGroupMembers: no current members.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		// Pre-flight: user does not exist.
		notFound := &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Reason: metav1.StatusReasonNotFound,
				Code:   http.StatusNotFound,
			},
		}
		userCache.EXPECT().Get("u-missing").Return(nil, notFound)

		srv := &SCIMServer{
			groupsCache: groupCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "add",
					"path": "members",
					"value": []map[string]any{
						{"value": "u-missing", "display": "missing.user"},
					},
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var resp404 Error
		err := json.Unmarshal(w.Body.Bytes(), &resp404)
		require.NoError(t, err)
		assert.Contains(t, resp404.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusNotFound, resp404.Status)
	})

	t.Run("remove member operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// Removing member.
		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
		}, nil)
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"}}},
			},
		}, false, nil)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(&v3.UserAttribute{}, nil)

		// For final getRancherGroupMembers call.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache:    groupCache,
			userCache:      userCache,
			userMGR:        userMGR,
			userAttributes: userAttrClient,
			getConfig:      testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "remove",
					"path": `members[value eq "u-mo773yttt4"]`,
				},
			},
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("replace externalId operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "old-external-id",
			Provider:    provider,
		}
		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		updatedGroup := existingGroup.DeepCopy()
		updatedGroup.ExternalID = "new-external-id"
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Update(gomock.Any()).Return(updatedGroup, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groups:      groupClient,
			groupsCache: groupCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "replace",
					"path":  "externalId",
					"value": "new-external-id",
				},
			},
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new-external-id", resp["externalId"])
	})

	t.Run("add externalId operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "old-external-id",
			Provider:    provider,
		}
		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		updatedGroup := existingGroup.DeepCopy()
		updatedGroup.ExternalID = "new-external-id"
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Update(gomock.Any()).Return(updatedGroup, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groups:      groupClient,
			groupsCache: groupCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "Add",
					"path":  "externalId",
					"value": "new-external-id",
				},
			},
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new-external-id", resp["externalId"])
	})

	t.Run("rejects externalId change in externalId mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "old-external-id",
			Provider:    provider,
		}
		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		srv := &SCIMServer{
			groupsCache: groupCache,
			getConfig: func(string) scimconfig.Config {
				return scimconfig.Config{GroupIDAttribute: scimconfig.GroupIDExternalID}
			},
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "Add",
					"path":  "externalId",
					"value": "new-external-id",
				},
			},
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Detail, "cannot be changed")
		assert.Equal(t, "mutability", resp.ScimType)
	})

	t.Run("allows externalId no-op in externalId mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "same-external-id",
			Provider:    provider,
		}
		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupCache,
			userCache:   userCache,
			getConfig: func(string) scimconfig.Config {
				return scimconfig.Config{GroupIDAttribute: scimconfig.GroupIDExternalID}
			},
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "Add",
					"path":  "externalId",
					"value": "same-external-id",
				},
			},
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "same-external-id", resp["externalId"])
	})

	t.Run("URN-prefixed replace externalId", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "old-external-id",
			Provider:    provider,
		}
		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		updatedGroup := existingGroup.DeepCopy()
		updatedGroup.ExternalID = "new-external-id"
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Update(gomock.Any()).Return(updatedGroup, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groups:      groupClient,
			groupsCache: groupCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "replace",
					"path":  "urn:ietf:params:scim:schemas:core:2.0:Group:externalId",
					"value": "new-external-id",
				},
			},
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new-external-id", resp["externalId"])
	})

	t.Run("URN-prefixed add members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-mo773yttt4"},
			PrincipalIDs: []string{"local://u-mo773yttt4", provider + "_user://john.doe"},
		}, nil).Times(2) // Once in resolveGroupMembers, once inside addGroupMember.
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{provider: {Items: []v3.Principal{}}},
		}, false, nil)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(&v3.UserAttribute{}, nil)

		// First call in resolveGroupMembers: the group has no members yet.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)
		// Second call for the final getRancherGroupMembers.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}},
		}, nil)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"}}},
			},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"john.doe"}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
			userAttributes:     userAttrClient,
			getConfig:          testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "add",
					"path": "urn:ietf:params:scim:schemas:core:2.0:Group:members",
					"value": []map[string]any{
						{"value": "u-mo773yttt4", "display": "john.doe"},
					},
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("URN-prefixed remove member", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
		}, nil)
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"}}},
			},
		}, false, nil)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(&v3.UserAttribute{}, nil)

		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache:    groupCache,
			userCache:      userCache,
			userMGR:        userMGR,
			userAttributes: userAttrClient,
			getConfig:      testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":   "remove",
					"path": `urn:ietf:params:scim:schemas:core:2.0:Group:members[value eq "u-mo773yttt4"]`,
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("URN-prefixed path with wrong resource type returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		srv := &SCIMServer{
			groupsCache: groupCache,
			getConfig:   testDefaultGetConfig,
		}

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "replace",
					"path":  "urn:ietf:params:scim:schemas:core:2.0:User:userName",
					"value": "test",
				},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewReader(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestListGroupsPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	// Create 5 groups with names that will sort as: g-aaa, g-bbb, g-ccc, g-ddd, g-eee.
	groups := []*v3.Group{
		{ObjectMeta: metav1.ObjectMeta{Name: "g-ccc", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Group C"},
		{ObjectMeta: metav1.ObjectMeta{Name: "g-aaa", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Group A"},
		{ObjectMeta: metav1.ObjectMeta{Name: "g-eee", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Group E"},
		{ObjectMeta: metav1.ObjectMeta{Name: "g-bbb", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Group B"},
		{ObjectMeta: metav1.ObjectMeta{Name: "g-ddd", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Group D"},
	}

	groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
	groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return(groups, nil).AnyTimes()

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

	srv := &SCIMServer{
		groupsCache:        groupsCache,
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
		getConfig:          testDefaultGetConfig,
	}

	tests := []struct {
		name             string
		queryString      string
		wantStatus       int
		wantTotalResults int
		wantItemsPerPage int
		wantStartIndex   int
		wantGroupIDs     []string // Expected group IDs in order.
	}{
		{
			name:             "default pagination returns all groups",
			queryString:      "excludedAttributes=members",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 5,
			wantStartIndex:   1,
			wantGroupIDs:     []string{"g-aaa", "g-bbb", "g-ccc", "g-ddd", "g-eee"},
		},
		{
			name:             "first page with count=2",
			queryString:      "count=2&excludedAttributes=members",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 2,
			wantStartIndex:   1,
			wantGroupIDs:     []string{"g-aaa", "g-bbb"},
		},
		{
			name:             "second page with count=2",
			queryString:      "startIndex=3&count=2&excludedAttributes=members",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 2,
			wantStartIndex:   3,
			wantGroupIDs:     []string{"g-ccc", "g-ddd"},
		},
		{
			name:             "last partial page",
			queryString:      "startIndex=5&count=2&excludedAttributes=members",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 1,
			wantStartIndex:   5,
			wantGroupIDs:     []string{"g-eee"},
		},
		{
			name:             "startIndex beyond total returns empty",
			queryString:      "startIndex=100&excludedAttributes=members",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 0,
			wantStartIndex:   100,
			wantGroupIDs:     []string{},
		},
		{
			name:             "count=0 returns empty resources with totalResults",
			queryString:      "count=0&excludedAttributes=members",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 0,
			wantStartIndex:   1,
			wantGroupIDs:     []string{},
		},
		{
			name:        "invalid startIndex returns error",
			queryString: "startIndex=abc",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "negative startIndex returns error",
			queryString: "startIndex=-1",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups?"+tt.queryString, nil)
			r.SetPathValue("provider", provider)
			w := httptest.NewRecorder()

			srv.ListGroups(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				var resp listResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)

				assert.Equal(t, tt.wantTotalResults, resp.TotalResults)
				assert.Equal(t, tt.wantItemsPerPage, resp.ItemsPerPage)
				assert.Equal(t, tt.wantStartIndex, resp.StartIndex)
				assert.Len(t, resp.Resources, len(tt.wantGroupIDs))

				// Verify the order of returned groups.
				for i, wantID := range tt.wantGroupIDs {
					resource := resp.Resources[i].(map[string]any)
					assert.Equal(t, wantID, resource["id"])
				}
			}
		})
	}
}

func TestListGroupsPaginationConsistency(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	// Create 10 groups.
	groups := make([]*v3.Group, 10)
	for i := range 10 {
		groups[i] = &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: fmt.Sprintf("g-%03d", i), Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: fmt.Sprintf("Group %03d", i),
		}
	}

	groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
	groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return(groups, nil).AnyTimes()

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

	srv := &SCIMServer{
		groupsCache:        groupsCache,
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
		getConfig:          testDefaultGetConfig,
	}

	// Collect all group IDs by paginating through all pages.
	pageSize := 3
	var allCollectedIDs []string

	for startIndex := 1; ; startIndex += pageSize {
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1-scim/%s/Groups?startIndex=%d&count=%d&excludedAttributes=members", provider, startIndex, pageSize), nil)
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.ListGroups(w, r)
		require.Equal(t, http.StatusOK, w.Code)
		var resp listResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		if len(resp.Resources) == 0 {
			break
		}

		for _, r := range resp.Resources {
			resource := r.(map[string]any)
			allCollectedIDs = append(allCollectedIDs, resource["id"].(string))
		}

		// Safety check to prevent infinite loop.
		if startIndex > 100 {
			t.Fatal("Too many iterations")
		}
	}

	// Verify we collected all 10 groups.
	assert.Len(t, allCollectedIDs, 10)

	// Verify no duplicates.
	seen := make(map[string]bool)
	for _, id := range allCollectedIDs {
		assert.False(t, seen[id], "Duplicate group ID found: %s", id)
		seen[id] = true
	}

	// Verify sorted order.
	for i := 1; i < len(allCollectedIDs); i++ {
		assert.True(t, allCollectedIDs[i-1] < allCollectedIDs[i],
			"Groups not in sorted order: %s should come before %s", allCollectedIDs[i-1], allCollectedIDs[i])
	}
}

func TestListGroupsWithExternalIdFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "azuread"

	groups := []*v3.Group{
		{ObjectMeta: metav1.ObjectMeta{Name: "g-aaa", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Engineering", ExternalID: "obj-111"},
		{ObjectMeta: metav1.ObjectMeta{Name: "g-bbb", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Marketing", ExternalID: "obj-222"},
		{ObjectMeta: metav1.ObjectMeta{Name: "g-ccc", Labels: map[string]string{authProviderLabel: provider}}, DisplayName: "Sales", ExternalID: "obj-333"},
	}

	groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
	groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return(groups, nil)

	srv := &SCIMServer{
		groupsCache: groupsCache,
		getConfig:   testDefaultGetConfig,
	}

	r := httptest.NewRequest(http.MethodGet, `/v1-scim/`+provider+`/Groups?filter=externalId%20eq%20%22obj-222%22&excludedAttributes=members`, nil)
	r.SetPathValue("provider", provider)
	w := httptest.NewRecorder()

	srv.ListGroups(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var resp listResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.TotalResults)
	require.Len(t, resp.Resources, 1)

	resource := resp.Resources[0].(map[string]any)
	assert.Equal(t, "g-bbb", resource["id"])
	assert.Equal(t, "Marketing", resource["displayName"])
}

func TestCreateGroup(t *testing.T) {
	provider := "okta"

	t.Run("creates group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{}, nil)

		createdGroup := &v3.Group{
			ObjectMeta: metav1.ObjectMeta{
				Name: "grp-abc123",
			},
			DisplayName: "Engineering",
			ExternalID:  "ext-eng-001",
		}
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(g *v3.Group) (*v3.Group, error) {
			assert.Equal(t, "Engineering", g.DisplayName)
			assert.Equal(t, "ext-eng-001", g.ExternalID)
			assert.Equal(t, provider, g.Labels[authProviderLabel])
			return createdGroup, nil
		})

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"externalId": "ext-eng-001"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)

		require.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, groupID, resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])
		assert.Equal(t, "ext-eng-001", resp["externalId"])

		meta, ok := resp["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, groupResource, meta["resourceType"])

		wantLocation := "/v1-scim/" + provider + "/Groups/" + groupID
		assert.Contains(t, meta["location"], wantLocation)
		assert.Contains(t, w.Header().Get("Location"), wantLocation)
	})

	t.Run("creates group with members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{}, nil)

		createdGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-abc123"},
			DisplayName: "Engineering",
			ExternalID:  "ext-eng-001",
		}
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Create(gomock.Any()).Return(createdGroup, nil)

		// Mock for resolveGroupMembers -> getRancherGroupMembers
		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
				Enabled:    &enabled,
			},
		}, nil)
		userCache.EXPECT().Get("u-user1").Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-user1"},
			Enabled:      &enabled,
			PrincipalIDs: []string{"local://u-user1", provider + "_user://user1"},
		}, nil).Times(2) // Once in resolveGroupMembers, once inside addGroupMember.

		userAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"user1"}},
			},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil) // getRancherGroupMembers
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-user1").Return(userAttr, false, nil)

		userAttributeClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttributeClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			// addGroupMember updates GroupPrincipals, not ExtraByProvider
			principals := attr.GroupPrincipals[provider].Items
			require.Len(t, principals, 1)
			assert.Equal(t, "Engineering", principals[0].DisplayName)
			return attr, nil
		})

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			groups:             groupClient,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
			userAttributes:     userAttributeClient,
			getConfig:          testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"externalId": "ext-eng-001",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)

		require.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		members, ok := resp["members"].([]any)
		require.True(t, ok)
		require.Len(t, members, 1)
	})

	t.Run("conflict when group already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-existing"},
			DisplayName: "Engineering",
		}
		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{existingGroup}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusConflict, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusConflict, resp.Status)

	})

	t.Run("conflict is case insensitive", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-existing"},
			DisplayName: "ENGINEERING",
		}
		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{existingGroup}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "engineering"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusConflict, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusConflict, resp.Status)
	})

	t.Run("invalid request body", func(t *testing.T) {
		srv := &SCIMServer{}

		body := `not valid json`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
	})

	t.Run("missing displayName", func(t *testing.T) {
		// ensureRancherGroup must not be called when displayName is absent.
		srv := &SCIMServer{}

		body := `{"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"]}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
		assert.Contains(t, resp.Detail, "displayName is required")
	})

	t.Run("error listing groups", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error creating group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{}, nil)

		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("create failed"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error resolving members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// The group is not listed or created, because the members are resolved first.
		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)

		// resolveGroupMembers -> getRancherGroupMembers calls userCache.List
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list failed"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)

		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error syncing members after the group is created", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)
		srv.groups.EXPECT().Create(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
			group = group.DeepCopy()
			group.Name = "grp-new"
			return group, nil
		})
		// The member resolves, but adding it fails.
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute("u-owned").Return(nil, false, fmt.Errorf("some error"))

		payload := map[string]any{"displayName": "NewGroup", "members": []map[string]any{{"value": "u-owned"}}}
		w := httptest.NewRecorder()
		srv.CreateGroup(w, newSCIMRequest(t, http.MethodPost, providerA, groupEndpoint, "", payload))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("rejects group type member in members list", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// The group is not listed or created, because the members are resolved first.
		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)

		// resolveGroupMembers -> getRancherGroupMembers needs List; the member type check rejects
		// group type before any Get.
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"members": [{"value": "grp-xyz", "display": "SubTeam", "type": "Group"}]
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
	})

	t.Run("member not found returns 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// The group is not listed or created, because the members are resolved first.
		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)

		// resolveGroupMembers -> getRancherGroupMembers; then the member Get finds user missing.
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)
		notFound := &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Reason: metav1.StatusReasonNotFound,
				Code:   http.StatusNotFound,
			},
		}
		userCache.EXPECT().Get("u-missing").Return(nil, notFound)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"members": [{"value": "u-missing", "display": "missing.user"}]
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)

		require.Equal(t, http.StatusNotFound, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusNotFound, resp.Status)
	})

	t.Run("conflict by displayName when the body has an id", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-existing", Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{existingGroup}, nil)

		// CreateGroup ignores the id in the body and finds the existing group by displayName.
		// It returns 409 Conflict without persisting any change. The mutability-gated update
		// belongs to UpdateGroup.
		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-existing",
			"displayName": "Engineering",
			"externalId": "new-ext-id"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusConflict, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusConflict, resp.Status)
	})

	t.Run("conflict when group found by displayName with different externalId", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-existing"},
			DisplayName: "Engineering",
			ExternalID:  "old-ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{existingGroup}, nil)

		// When a group with the same displayName already exists, CreateGroup returns
		// 409 Conflict without mutating the existing group's externalId.
		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"externalId": "new-ext-id"
		}`
		r := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		w := httptest.NewRecorder()

		srv.CreateGroup(w, r)
		require.Equal(t, http.StatusConflict, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusConflict, resp.Status)
	})
}

func TestGetGroup(t *testing.T) {
	provider := "okta"

	t.Run("returns group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-eng-001",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		// getRancherGroupMembers needs userCache
		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
				Enabled:    &enabled,
			},
		}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"user1"}},
			},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{
					{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"},
				}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			getConfig:          testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, groupID, resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])
		assert.Equal(t, "ext-eng-001", resp["externalId"])

		members, ok := resp["members"].([]any)
		require.True(t, ok)
		require.Len(t, members, 1)
		member := members[0].(map[string]any)
		assert.Equal(t, "u-user1", member["value"])
		assert.Equal(t, "user1", member["display"])
		assert.Equal(t, userResource, member["type"])
	})

	t.Run("returns group without members when excluded", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-eng-001",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		// No userCache calls expected when members excluded

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID+"?excludedAttributes=members,other_attribute", nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, groupID, resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])

		// members should not be in response
		_, hasMembers := resp["members"]
		assert.False(t, hasMembers)
	})

	t.Run("returns empty members array when no members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		members, ok := resp["members"].([]any)
		require.True(t, ok)
		assert.Empty(t, members)
	})

	t.Run("group not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-notfound"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, apierrors.NewNotFound(v3.Resource("group"), groupID))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusNotFound, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusNotFound, resp.Status)
	})

	t.Run("error getting group from cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error getting group members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("skips system users when getting members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{
				ObjectMeta:   metav1.ObjectMeta{Name: "u-system"},
				PrincipalIDs: []string{"system://local"},
				Enabled:      &enabled,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "u-normal"},
				Enabled:    &enabled,
			},
		}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-normal").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-normal"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"normal-user"}},
			},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{
					{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"},
				}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			getConfig:          testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.GetGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		members, ok := resp["members"].([]any)
		require.True(t, ok)
		require.Len(t, members, 1)
		member := members[0].(map[string]any)
		assert.Equal(t, "u-normal", member["value"])
		assert.Equal(t, userResource, member["type"])
	})
}

func TestUpdateGroup(t *testing.T) {
	provider := "okta"

	t.Run("updates group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		// externalId is unchanged so ensureRancherGroup does not call groups.Update.
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// syncGroupMembers needs userCache for getRancherGroupMembers
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": []
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, groupID, resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])
		assert.Equal(t, "ext-id", resp["externalId"])

		meta, ok := resp["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, groupResource, meta["resourceType"])

		wantLocation := "/v1-scim/" + provider + "/Groups/" + groupID
		assert.Contains(t, meta["location"], wantLocation)
		assert.Contains(t, w.Header().Get("Location"), wantLocation)
	})

	t.Run("updates externalId when it changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "old-ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		updatedGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
			ExternalID:  "new-ext-id",
		}
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(g *v3.Group) (*v3.Group, error) {
			assert.Equal(t, "new-ext-id", g.ExternalID)
			return updatedGroup, nil
		})

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "new-ext-id",
			"members": []
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, groupID, resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])
		assert.Equal(t, "new-ext-id", resp["externalId"])
	})

	t.Run("rejects externalId change with externalId config", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "old-ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig: func(string) scimconfig.Config {
				return scimconfig.Config{GroupIDAttribute: scimconfig.GroupIDExternalID}
			},
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "new-ext-id",
			"members": []
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Detail, "cannot be changed")
		assert.Equal(t, "mutability", resp.ScimType)
	})

	t.Run("updates displayName with externalId config", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Old Name",
			ExternalID:  "ext-123",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(g *v3.Group) (*v3.Group, error) {
			assert.Equal(t, "New Name", g.DisplayName)
			return g, nil
		})

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig: func(string) scimconfig.Config {
				return scimconfig.Config{GroupIDAttribute: scimconfig.GroupIDExternalID}
			},
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "New Name",
			"externalId": "ext-123",
			"members": []
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "New Name", resp["displayName"])
	})

	t.Run("rejects displayName change with default config", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Old Name",
			ExternalID:  "ext-123",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "New Name",
			"externalId": "ext-123",
			"members": []
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Detail, "cannot be changed")
	})

	t.Run("updates group with members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// No update needed when externalId unchanged
		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: "u-user1"}, Enabled: &enabled},
		}, nil)
		userCache.EXPECT().Get("u-user1").Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-user1"},
			Enabled:      &enabled,
			PrincipalIDs: []string{"local://u-user1", provider + "_user://user1"},
		}, nil).Times(2) // Once in resolveGroupMembers, once inside addGroupMember.

		userAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"user1"}},
			},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil)
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-user1").Return(userAttr, false, nil)

		userAttributeClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttributeClient.EXPECT().Update(gomock.Any()).Return(userAttr, nil)

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
			userAttributes:     userAttributeClient,
			getConfig:          testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		members, ok := resp["members"].([]any)
		require.True(t, ok)
		require.Len(t, members, 1)
	})

	t.Run("mismatched group id", func(t *testing.T) {
		srv := &SCIMServer{}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-different",
			"displayName": "Engineering"
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/grp-abc123", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", "grp-abc123")
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
	})

	t.Run("invalid request body", func(t *testing.T) {
		srv := &SCIMServer{}

		body := `not valid json`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/grp-abc123", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", "grp-abc123")
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
	})

	t.Run("missing displayName", func(t *testing.T) {
		// ensureRancherGroup must not be called when displayName is absent.
		srv := &SCIMServer{}

		body := `{"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"], "id": "grp-abc123"}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/grp-abc123", bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", "grp-abc123")
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusBadRequest, resp.Status)
		assert.Contains(t, resp.Detail, "displayName is required")
	})

	t.Run("error ensuring group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering"
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error resolving members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error syncing members after the group is renamed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)
		srv.getConfig = externalIDGroupConfig
		srv.groups.EXPECT().Update(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
			assert.Equal(t, "GroupA renamed", group.DisplayName)
			return group, nil
		})
		// The member resolves, but adding it fails.
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute("u-owned").Return(nil, false, fmt.Errorf("some error"))

		payload := map[string]any{
			"id":          "grp-a",
			"displayName": "GroupA renamed",
			"externalId":  "ext-grp-a",
			"members":     []map[string]any{{"value": "u-owned"}},
		}
		w := httptest.NewRecorder()
		srv.UpdateGroup(w, newSCIMRequest(t, http.MethodPut, providerA, groupEndpoint, "grp-a", payload))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("removes members not in update", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// First call for getRancherGroupMembers
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: "u-user1"}, Enabled: &enabled},
		}, nil)
		// Second call for removeGroupMember
		userCache.EXPECT().Get("u-user1").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			Enabled:    &enabled,
		}, nil)

		userAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"user1"}},
			},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{
					{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"},
				}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil)
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-user1").Return(userAttr, false, nil)

		userAttributeClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttributeClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			// Verify the member was removed
			principals := attr.GroupPrincipals[provider].Items
			assert.Empty(t, principals)
			return attr, nil
		})

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
			userAttributes:     userAttributeClient,
			getConfig:          testDefaultGetConfig,
		}

		// Update with empty members list should remove existing member
		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": []
		}`
		r := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.UpdateGroup(w, r)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestDeleteGroup(t *testing.T) {
	provider := "okta"

	t.Run("deletes group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Delete(groupID, gomock.Any()).Return(nil)

		// removeAllGroupMembers needs userCache
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.DeleteGroup(w, r)
		require.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("removes members before deleting group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Delete(groupID, gomock.Any()).Return(nil)

		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: "u-user1"}, Enabled: &enabled},
		}, nil)
		userCache.EXPECT().Get("u-user1").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			Enabled:    &enabled,
		}, nil)

		userAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"user1"}},
			},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{
					{ObjectMeta: metav1.ObjectMeta{Name: groupPrincipalName(provider, "Engineering")}, DisplayName: "Engineering"},
				}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil)
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureAndGetUserAttribute("u-user1").Return(userAttr, false, nil)

		userAttributeClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttributeClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			// Verify member was removed from group
			principals := attr.GroupPrincipals[provider].Items
			assert.Empty(t, principals)
			return attr, nil
		})

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			groups:             groupClient,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
			userAttributes:     userAttributeClient,
			getConfig:          testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.DeleteGroup(w, r)
		require.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("group not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-notfound"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, apierrors.NewNotFound(v3.Resource("group"), groupID))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.DeleteGroup(w, r)
		require.Equal(t, http.StatusNotFound, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusNotFound, resp.Status)
	})

	t.Run("error getting group from cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.DeleteGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error removing group members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.DeleteGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})

	t.Run("error deleting group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID, Labels: map[string]string{authProviderLabel: provider}},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Delete(groupID, gomock.Any()).Return(fmt.Errorf("delete failed"))

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
			getConfig:   testDefaultGetConfig,
		}

		r := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		r.SetPathValue("provider", provider)
		r.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		srv.DeleteGroup(w, r)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var resp Error
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Schemas, errorSchemaID)
		assert.Equal(t, http.StatusInternalServerError, resp.Status)
	})
}

func TestGroupHandlersOwnership(t *testing.T) {
	for _, id := range []string{"grp-b", "grp-missing"} {
		putPayload := map[string]any{"id": id, "displayName": "GroupB", "externalId": "ext-grp-b", "members": []map[string]any{}}
		patchPayload := map[string]any{
			"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{{"op": "replace", "path": "externalId", "value": "changed"}},
		}

		for _, tt := range []struct {
			method  string
			handler func(*SCIMServer) http.HandlerFunc
			body    any
		}{
			{method: http.MethodGet, handler: func(s *SCIMServer) http.HandlerFunc { return s.GetGroup }},
			{method: http.MethodPut, handler: func(s *SCIMServer) http.HandlerFunc { return s.UpdateGroup }, body: putPayload},
			{method: http.MethodPatch, handler: func(s *SCIMServer) http.HandlerFunc { return s.PatchGroup }, body: patchPayload},
			{method: http.MethodDelete, handler: func(s *SCIMServer) http.HandlerFunc { return s.DeleteGroup }},
		} {
			t.Run(tt.method+" "+id+" returns not found and writes nothing", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				srv := newOwnershipFixtures().server(ctrl)

				w := httptest.NewRecorder()
				tt.handler(srv.SCIMServer)(w, newSCIMRequest(t, tt.method, providerA, groupEndpoint, id, tt.body))
				require.Equal(t, http.StatusNotFound, w.Code)

				var resp Error
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "Group "+id+" not found", resp.Detail)
			})
		}
	}
}

func TestCreateGroupIgnoresBodyID(t *testing.T) {
	for _, id := range []string{"grp-b", "grp-a", "grp-missing"} {
		for _, tt := range []struct {
			displayName string
			wantStatus  int
		}{
			{displayName: "NewGroup", wantStatus: http.StatusCreated},
			{displayName: "GroupA", wantStatus: http.StatusConflict},
			{displayName: "GroupB", wantStatus: http.StatusCreated},
		} {
			t.Run("id "+id+" and displayName "+tt.displayName, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				f := newOwnershipFixtures()
				srv := f.server(ctrl)

				// The group is looked up by displayName within provider A's groups, never by id.
				groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
				groupsCache.EXPECT().List(labels.Set{authProviderLabel: providerA}.AsSelector()).Return([]*v3.Group{f.groups["grp-a"]}, nil)
				srv.groupsCache = groupsCache

				if tt.wantStatus == http.StatusCreated {
					srv.groups.EXPECT().Create(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
						assert.Equal(t, providerA, group.Labels[authProviderLabel])
						assert.Equal(t, tt.displayName, group.DisplayName)
						group = group.DeepCopy()
						group.Name = "grp-new"
						return group, nil
					})
				}

				payload := map[string]any{"id": id, "displayName": tt.displayName}
				w := httptest.NewRecorder()
				srv.CreateGroup(w, newSCIMRequest(t, http.MethodPost, providerA, groupEndpoint, "", payload))
				require.Equal(t, tt.wantStatus, w.Code)

				if tt.wantStatus == http.StatusCreated {
					var resp map[string]any
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
					assert.Equal(t, "grp-new", resp["id"])
				}
			})
		}
	}
}

// memberIDs returns the member IDs from a SCIM group resource.
func memberIDs(t *testing.T, resource map[string]any) []string {
	t.Helper()

	members, ok := resource["members"].([]any)
	require.True(t, ok)

	var ids []string
	for _, member := range members {
		ids = append(ids, member.(map[string]any)["value"].(string))
	}
	return ids
}

// assertGroupARemoved returns a DoAndReturn function for userAttributes.Update that checks grp-a's
// principal was removed.
// When the test ends, it checks that the updated users are exactly the given ones.
func assertGroupARemoved(t *testing.T, users ...string) func(*v3.UserAttribute) (*v3.UserAttribute, error) {
	var updated []string
	t.Cleanup(func() { assert.ElementsMatch(t, users, updated) })

	return func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
		updated = append(updated, attr.Name)
		assert.Empty(t, attr.GroupPrincipals[providerA].Items)
		return attr, nil
	}
}

func TestGroupExistingMembersNotOwned(t *testing.T) {
	// grp-a has three members: u-locallinked is owned; u-admin and u-othermember aren't.
	t.Run("GET shows members who aren't owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)

		w := httptest.NewRecorder()
		srv.GetGroup(w, newSCIMRequest(t, http.MethodGet, providerA, groupEndpoint, "grp-a", nil))
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, []string{"u-admin", "u-locallinked", "u-othermember"}, memberIDs(t, resp))
	})

	t.Run("list shows members who aren't owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)

		w := httptest.NewRecorder()
		srv.ListGroups(w, newSCIMRequest(t, http.MethodGet, providerA, groupEndpoint, "", nil))
		require.Equal(t, http.StatusOK, w.Code)

		var resp listResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Resources, 2) // grp-a and grp-a2.
		assert.Equal(t, []string{"u-admin", "u-locallinked", "u-othermember"}, memberIDs(t, resp.Resources[0].(map[string]any)))
	})

	t.Run("PUT keeps members who aren't owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		f := newOwnershipFixtures()
		srv := f.server(ctrl)
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute(gomock.Any()).DoAndReturn(f.ensureAndGetUserAttribute).Times(3)

		payload := map[string]any{
			"id":          "grp-a",
			"displayName": "GroupA",
			"externalId":  "ext-grp-a",
			"members":     []map[string]any{{"value": "u-admin"}, {"value": "u-othermember"}, {"value": "u-locallinked"}},
		}
		w := httptest.NewRecorder()
		srv.UpdateGroup(w, newSCIMRequest(t, http.MethodPut, providerA, groupEndpoint, "grp-a", payload))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PUT that keeps the default admin and adds a system user returns not found and writes nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)

		payload := map[string]any{
			"id":          "grp-a",
			"displayName": "GroupA",
			"externalId":  "ext-grp-a",
			"members":     []map[string]any{{"value": "u-admin"}, {"value": "u-othermember"}, {"value": "u-locallinked"}, {"value": "u-system"}},
		}
		w := httptest.NewRecorder()
		srv.UpdateGroup(w, newSCIMRequest(t, http.MethodPut, providerA, groupEndpoint, "grp-a", payload))
		require.Equal(t, http.StatusNotFound, w.Code)

		var resp Error
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "User u-system not found", resp.Detail)
	})

	t.Run("PATCH add keeps members who aren't owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		f := newOwnershipFixtures()
		srv := f.server(ctrl)
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute(gomock.Any()).DoAndReturn(f.ensureAndGetUserAttribute).Times(2)

		payload := map[string]any{
			"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{{"op": "add", "path": "members", "value": []map[string]any{{"value": "u-admin"}, {"value": "u-othermember"}}}},
		}
		w := httptest.NewRecorder()
		srv.PatchGroup(w, newSCIMRequest(t, http.MethodPatch, providerA, groupEndpoint, "grp-a", payload))
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, []string{"u-admin", "u-locallinked", "u-othermember"}, memberIDs(t, resp))
	})

	t.Run("PATCH remove takes out members who aren't owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		f := newOwnershipFixtures()
		srv := f.server(ctrl)
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute(gomock.Any()).DoAndReturn(f.ensureAndGetUserAttribute).Times(2)
		srv.userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(assertGroupARemoved(t, "u-admin", "u-othermember")).Times(2)

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{"op": "remove", "path": `members[value eq "u-admin"]`},
				{"op": "remove", "path": `members[value eq "u-othermember"]`},
			},
		}
		w := httptest.NewRecorder()
		srv.PatchGroup(w, newSCIMRequest(t, http.MethodPatch, providerA, groupEndpoint, "grp-a", payload))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PUT without members who aren't owned takes them out", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		f := newOwnershipFixtures()
		srv := f.server(ctrl)
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute(gomock.Any()).DoAndReturn(f.ensureAndGetUserAttribute).Times(3)
		srv.userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(assertGroupARemoved(t, "u-admin", "u-othermember")).Times(2)

		payload := map[string]any{"id": "grp-a", "displayName": "GroupA", "externalId": "ext-grp-a", "members": []map[string]any{{"value": "u-locallinked"}}}
		w := httptest.NewRecorder()
		srv.UpdateGroup(w, newSCIMRequest(t, http.MethodPut, providerA, groupEndpoint, "grp-a", payload))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DELETE takes out members who aren't owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		f := newOwnershipFixtures()
		srv := f.server(ctrl)
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute(gomock.Any()).DoAndReturn(f.ensureAndGetUserAttribute).Times(3)
		srv.userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(assertGroupARemoved(t, "u-admin", "u-othermember", "u-locallinked")).Times(3)
		srv.groups.EXPECT().Delete("grp-a", gomock.Any()).Return(nil)

		w := httptest.NewRecorder()
		srv.DeleteGroup(w, newSCIMRequest(t, http.MethodDelete, providerA, groupEndpoint, "grp-a", nil))
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("POST keeps a member who isn't owned but already holds the group principal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		f := newOwnershipFixtures()
		// The default admin got the new group's principal from a login to provider A.
		f.attrs["u-admin"].GroupPrincipals[providerA] = v3.Principals{Items: []v3.Principal{{
			ObjectMeta:  metav1.ObjectMeta{Name: groupPrincipalName(providerA, "NewGroup")},
			DisplayName: "NewGroup",
		}}}
		srv := f.server(ctrl)
		srv.groups.EXPECT().Create(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
			group = group.DeepCopy()
			group.Name = "grp-new"
			return group, nil
		})
		srv.userMGR.EXPECT().EnsureAndGetUserAttribute("u-admin").DoAndReturn(f.ensureAndGetUserAttribute)

		payload := map[string]any{"displayName": "NewGroup", "members": []map[string]any{{"value": "u-admin"}}}
		w := httptest.NewRecorder()
		srv.CreateGroup(w, newSCIMRequest(t, http.MethodPost, providerA, groupEndpoint, "", payload))
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestGroupMemberAdditionsOwnership(t *testing.T) {
	requests := []struct {
		name string
		// send runs the request that adds the member to a group the member isn't in.
		send func(t *testing.T, srv *ownershipServer, member string) *httptest.ResponseRecorder
		// groupWrite sets up the group write a successful request makes.
		groupWrite func(t *testing.T, srv *ownershipServer)
		// principal is the group principal a successful request gives the member.
		principal string
	}{
		{
			// The PATCH also changes externalId, so a write before the member check would show.
			name: "PATCH add",
			send: func(t *testing.T, srv *ownershipServer, member string) *httptest.ResponseRecorder {
				payload := map[string]any{
					"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
					"Operations": []map[string]any{
						{"op": "replace", "path": "externalId", "value": "ext-grp-a2-new"},
						{"op": "add", "path": "members", "value": []map[string]any{{"value": member}}},
					},
				}
				w := httptest.NewRecorder()
				srv.PatchGroup(w, newSCIMRequest(t, http.MethodPatch, providerA, groupEndpoint, "grp-a2", payload))
				return w
			},
			groupWrite: func(t *testing.T, srv *ownershipServer) {
				srv.groups.EXPECT().Update(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
					assert.Equal(t, "ext-grp-a2-new", group.ExternalID)
					return group, nil
				})
			},
			principal: groupPrincipalName(providerA, "GroupA2"),
		},
		{
			name: "POST",
			send: func(t *testing.T, srv *ownershipServer, member string) *httptest.ResponseRecorder {
				payload := map[string]any{"displayName": "NewGroup", "members": []map[string]any{{"value": member}}}
				w := httptest.NewRecorder()
				srv.CreateGroup(w, newSCIMRequest(t, http.MethodPost, providerA, groupEndpoint, "", payload))
				return w
			},
			groupWrite: func(t *testing.T, srv *ownershipServer) {
				srv.groups.EXPECT().Create(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
					group = group.DeepCopy()
					group.Name = "grp-new"
					return group, nil
				})
			},
			principal: groupPrincipalName(providerA, "NewGroup"),
		},
		{
			// The PUT also renames the group, so a write before the member check would show.
			name: "PUT",
			send: func(t *testing.T, srv *ownershipServer, member string) *httptest.ResponseRecorder {
				srv.getConfig = externalIDGroupConfig
				payload := map[string]any{
					"id":          "grp-a2",
					"displayName": "GroupA2 renamed",
					"externalId":  "ext-grp-a2",
					"members":     []map[string]any{{"value": member}},
				}
				w := httptest.NewRecorder()
				srv.UpdateGroup(w, newSCIMRequest(t, http.MethodPut, providerA, groupEndpoint, "grp-a2", payload))
				return w
			},
			groupWrite: func(t *testing.T, srv *ownershipServer) {
				srv.groups.EXPECT().Update(gomock.Any()).DoAndReturn(func(group *v3.Group) (*v3.Group, error) {
					assert.Equal(t, "GroupA2 renamed", group.DisplayName)
					return group, nil
				})
			},
			principal: groupPrincipalName(providerA, "ext-grp-a2"),
		},
	}

	for _, req := range requests {
		// u-admin and u-system hold a provider A principal but aren't owned.
		for _, member := range []string{"u-other", "u-local", "u-admin", "u-system", "u-missing"} {
			t.Run(req.name+" with "+member+" returns not found and writes nothing", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				srv := newOwnershipFixtures().server(ctrl)

				w := req.send(t, srv, member)
				require.Equal(t, http.StatusNotFound, w.Code)

				var resp Error
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "User "+member+" not found", resp.Detail)
			})
		}

		t.Run(req.name+" with a user cache error returns 500 and writes nothing", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			srv := newOwnershipFixtures().server(ctrl)
			userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
			userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)
			userCache.EXPECT().Get("u-owned").Return(nil, fmt.Errorf("some error"))
			srv.userCache = userCache

			w := req.send(t, srv, "u-owned")
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run(req.name+" with a local user who linked the provider adds the member", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			f := newOwnershipFixtures()
			srv := f.server(ctrl)
			req.groupWrite(t, srv)
			srv.userMGR.EXPECT().EnsureAndGetUserAttribute("u-locallinkednm").DoAndReturn(f.ensureAndGetUserAttribute)
			srv.userAttributes.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
				assert.Equal(t, "u-locallinkednm", attr.Name)
				require.Len(t, attr.GroupPrincipals[providerA].Items, 1)
				assert.Equal(t, req.principal, attr.GroupPrincipals[providerA].Items[0].Name)
				return attr, nil
			})

			w := req.send(t, srv, "u-locallinkednm")
			assert.Less(t, w.Code, http.StatusMultipleChoices)
		})
	}
}

func TestGroupMemberAdditionsAllOrNothing(t *testing.T) {
	// u-owned could be added, but u-other can't, so the request fails as a whole and nothing is
	// written.
	members := []map[string]any{{"value": "u-owned"}, {"value": "u-other"}}

	t.Run("POST", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)

		payload := map[string]any{"displayName": "NewGroup", "members": members}
		w := httptest.NewRecorder()
		srv.CreateGroup(w, newSCIMRequest(t, http.MethodPost, providerA, groupEndpoint, "", payload))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("PUT", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)

		payload := map[string]any{"id": "grp-a", "displayName": "GroupA", "externalId": "ext-grp-a", "members": members}
		w := httptest.NewRecorder()
		srv.UpdateGroup(w, newSCIMRequest(t, http.MethodPut, providerA, groupEndpoint, "grp-a", payload))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("PATCH that also removes a member", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		srv := newOwnershipFixtures().server(ctrl)

		payload := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{"op": "remove", "path": `members[value eq "u-locallinked"]`},
				{"op": "add", "path": "members", "value": members},
			},
		}
		w := httptest.NewRecorder()
		srv.PatchGroup(w, newSCIMRequest(t, http.MethodPatch, providerA, groupEndpoint, "grp-a", payload))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
