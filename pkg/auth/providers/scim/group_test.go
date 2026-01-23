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

func TestCreateGroup(t *testing.T) {
	provider := "okta"

	t.Run("creates group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

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
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"externalId": "ext-eng-001"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "grp-abc123", resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])
		assert.Equal(t, "ext-eng-001", resp["externalId"])
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

		// Mock for syncGroupMembers -> getRancherGroupMembers
		enabled := true
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
				Enabled:    &enabled,
			},
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
				provider: {Items: []v3.Principal{}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil).Times(2) // Once for getRancherGroupMembers, once for addGroupMember

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
			userAttributes:     userAttributeClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"externalId": "ext-eng-001",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
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
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "engineering"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("invalid request body", func(t *testing.T) {
		srv := &SCIMServer{}

		body := `not valid json`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("error listing groups", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
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
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error syncing members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().List(labels.Set{authProviderLabel: provider}.AsSelector()).Return([]*v3.Group{}, nil)

		createdGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-abc123"},
			DisplayName: "Engineering",
		}
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Create(gomock.Any()).Return(createdGroup, nil)

		// getRancherGroupMembers calls userCache.List
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list failed"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
			userCache:   userCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("conflict when group found by ID", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: "grp-existing"},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get("grp-existing").Return(existingGroup, nil)

		// When ID is provided and exists, ensureRancherGroup returns the existing group
		// with created=false, which triggers a 409 Conflict.

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-existing",
			"displayName": "Engineering",
			"externalId": "new-ext-id"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
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

		// When found by displayName with different externalId, the group is updated
		// but created=false is returned, which triggers a 409 Conflict.
		groupClient := fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl)
		groupClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(g *v3.Group) (*v3.Group, error) {
			assert.Equal(t, "new-ext-id", g.ExternalID)
			return g, nil
		})

		srv := &SCIMServer{
			groupsCache: groupsCache,
			groups:      groupClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"displayName": "Engineering",
			"externalId": "new-ext-id"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Groups", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateGroup(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestGetGroup(t *testing.T) {
	provider := "okta"

	t.Run("returns group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
					{DisplayName: "Engineering"},
				}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
	})

	t.Run("returns group without members when excluded", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
			ExternalID:  "ext-eng-001",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		// No userCache calls expected when members excluded

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID+"?excludedAttributes=members", nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("error getting group from cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error getting group members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("skips system users when getting members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
					{DisplayName: "Engineering"},
				}},
			},
		}, nil)

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.GetGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		members, ok := resp["members"].([]any)
		require.True(t, ok)
		require.Len(t, members, 1)
		member := members[0].(map[string]any)
		assert.Equal(t, "u-normal", member["value"])
	})
}

func TestUpdateGroup(t *testing.T) {
	provider := "okta"

	t.Run("updates group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
			ExternalID:  "ext-id",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		// ensureRancherGroup with ID just returns from cache (no update logic)
		groupsCache.EXPECT().Get(groupID).Return(existingGroup, nil)

		// syncGroupMembers needs userCache for getRancherGroupMembers
		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": []
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, groupID, resp["id"])
		assert.Equal(t, "Engineering", resp["displayName"])
		assert.Equal(t, "ext-id", resp["externalId"])
	})

	t.Run("updates group with members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
			ObjectMeta: metav1.ObjectMeta{Name: "u-user1"},
			Enabled:    &enabled,
		}, nil)

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
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil).Times(2)

		userAttributeClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttributeClient.EXPECT().Update(gomock.Any()).Return(userAttr, nil)

		srv := &SCIMServer{
			groupsCache:        groupsCache,
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttributeClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
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
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/grp-abc123", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": "grp-abc123"})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid request body", func(t *testing.T) {
		srv := &SCIMServer{}

		body := `not valid json`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/grp-abc123", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": "grp-abc123"})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("error ensuring group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering"
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error syncing members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": [{"value": "u-user1", "display": "user1"}]
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("removes members not in update", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		existingGroup := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
					{DisplayName: "Engineering"},
				}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil).Times(2)

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
			userAttributes:     userAttributeClient,
		}

		// Update with empty members list should remove existing member
		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
			"id": "grp-abc123",
			"displayName": "Engineering",
			"externalId": "ext-id",
			"members": []
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Groups/"+groupID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.UpdateGroup(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestDeleteGroup(t *testing.T) {
	provider := "okta"

	t.Run("deletes group successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.DeleteGroup(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("removes members before deleting group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
					{DisplayName: "Engineering"},
				}},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-user1").Return(userAttr, nil).Times(2)

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
			userAttributes:     userAttributeClient,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.DeleteGroup(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("group not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-notfound"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, apierrors.NewNotFound(v3.Resource("group"), groupID))

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.DeleteGroup(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("error getting group from cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.DeleteGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error removing group members", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
			DisplayName: "Engineering",
		}

		groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
		groupsCache.EXPECT().Get(groupID).Return(group, nil)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("list error"))

		srv := &SCIMServer{
			groupsCache: groupsCache,
			userCache:   userCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.DeleteGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error deleting group", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		groupID := "grp-abc123"
		group := &v3.Group{
			ObjectMeta:  metav1.ObjectMeta{Name: groupID},
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
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Groups/"+groupID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": groupID})
		rec := httptest.NewRecorder()

		srv.DeleteGroup(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
