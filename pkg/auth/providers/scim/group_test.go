package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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
							{DisplayName: groupName},
							{DisplayName: "Other Group"},
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
							{DisplayName: "Different Group"},
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
	}

	members, err := srv.getRancherGroupMembers(provider, groupName)

	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "u-mo773yttt4", members[0].Value)
	assert.Equal(t, "john.doe", members[0].Display)
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
							{DisplayName: "Engineering"},
							{DisplayName: "Architects"},
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
							{DisplayName: "Engineering"},
							{DisplayName: "Developers"},
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
	}

	groups, err := srv.getAllRancherGroupMembers(provider)

	require.NoError(t, err)
	require.Len(t, groups, 3) // Engineering, Architects, Developers.

	// Verify Engineering group has 2 members.
	engineers := groups["Engineering"]
	require.Len(t, engineers, 2)

	// Check both members are present (order may vary).
	memberNames := []string{engineers[0].Value, engineers[1].Value}
	assert.Contains(t, memberNames, "u-mo773yttt4")
	assert.Contains(t, memberNames, "u-yypnjwjmkq")

	// Verify Architects group has 1 member.
	architects := groups["Architects"]
	require.Len(t, architects, 1)
	assert.Equal(t, "u-mo773yttt4", architects[0].Value)
	assert.Equal(t, "john.doe", architects[0].Display)

	// Verify Developers group has 1 member.
	developers := groups["Developers"]
	require.Len(t, developers, 1)
	assert.Equal(t, "u-yypnjwjmkq", developers[0].Value)
	assert.Equal(t, "jane.smith", developers[0].Display)
}

func TestSyncGroupMembers(t *testing.T) {
	provider := "okta"
	groupName := "Engineering"

	t.Run("adds new members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// Current state: no members.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		// Adding one member.
		newMember := SCIMMember{Value: "u-mo773yttt4", Display: "john.doe"}
		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(existingAttr, nil)

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
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		err := srv.syncGroupMembers(provider, groupName, []SCIMMember{newMember})
		require.NoError(t, err)
	})

	t.Run("removes members not in new list", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		// Current state: one member.
		existingUser := &v3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}}
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{existingUser}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {
					Items: []v3.Principal{{DisplayName: groupName}},
				},
			},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"john.doe"}},
			},
		}

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(existingAttr, nil)

		// Remove the member.
		userCache.EXPECT().Get("u-mo773yttt4").Return(existingUser, nil)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(existingAttr, nil)

		updatedAttr := existingAttr.DeepCopy()
		updatedAttr.GroupPrincipals[provider] = v3.Principals{Items: []v3.Principal{}}

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(updatedAttr, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		err := srv.syncGroupMembers(provider, groupName, []SCIMMember{})
		require.NoError(t, err)
	})
}

func TestApplyReplaceGroup(t *testing.T) {
	t.Run("replaces externalId", func(t *testing.T) {
		group := &v3.Group{ExternalID: "old-id"}
		op := patchOp{Op: "replace", Path: "externalId", Value: "new-id"}

		updated, err := applyReplaceGroup(group, op)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("no update when value same", func(t *testing.T) {
		group := &v3.Group{ExternalID: "same-id"}
		op := patchOp{Op: "replace", Path: "externalId", Value: "same-id"}

		updated, err := applyReplaceGroup(group, op)

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

		updated, err := applyReplaceGroup(group, op)

		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "new-id", group.ExternalID)
	})

	t.Run("rejects unsupported path", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{Op: "replace", Path: "unsupported", Value: "value"}

		updated, err := applyReplaceGroup(group, op)

		require.Error(t, err)
		assert.False(t, updated)
	})

	t.Run("rejects invalid value type", func(t *testing.T) {
		group := &v3.Group{}
		op := patchOp{Op: "replace", Path: "displayName", Value: 123}

		updated, err := applyReplaceGroup(group, op)

		require.Error(t, err)
		assert.False(t, updated)
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
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// Adding member.
		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
		}, nil)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{provider: {Items: []v3.Principal{}}},
		}, nil)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(&v3.UserAttribute{}, nil)

		// For final getRancherGroupMembers call.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"}},
		}, nil)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{{DisplayName: "Engineering"}}},
			},
			ExtraByProvider: map[string]map[string][]string{
				provider: {"username": {"john.doe"}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
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
		r = mux.SetURLVars(r, map[string]string{"provider": provider, "id": groupID})

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Engineering", response["displayName"])

		// Members should be present in response.
		if members, ok := response["members"].([]any); ok {
			// Members list is returned (could be empty based on mocks).
			assert.NotNil(t, members)
		}
	})

	t.Run("remove member operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
			Provider:    provider,
		}
		groupCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// Removing member.
		userCache.EXPECT().Get("u-mo773yttt4").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
		}, nil)
		userAttributeCache.EXPECT().Get("u-mo773yttt4").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-mo773yttt4"},
			GroupPrincipals: map[string]v3.Principals{
				provider: {Items: []v3.Principal{{DisplayName: "Engineering"}}},
			},
		}, nil)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(&v3.UserAttribute{}, nil)

		// For final getRancherGroupMembers call.
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache:        groupCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
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
		r = mux.SetURLVars(r, map[string]string{"provider": provider, "id": groupID})

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("replace externalId operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
		r = mux.SetURLVars(r, map[string]string{"provider": provider, "id": groupID})

		srv.PatchGroup(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "new-external-id", response["externalId"])
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
			req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups?"+tt.queryString, nil)
			req = mux.SetURLVars(req, map[string]string{"provider": provider})
			rec := httptest.NewRecorder()

			srv.ListGroups(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusOK {
				var resp ListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
	for i := 0; i < 10; i++ {
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
	}

	// Collect all group IDs by paginating through all pages.
	pageSize := 3
	var allCollectedIDs []string

	for startIndex := 1; ; startIndex += pageSize {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1-scim/%s/Groups?startIndex=%d&count=%d&excludedAttributes=members", provider, startIndex, pageSize), nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.ListGroups(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp ListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
