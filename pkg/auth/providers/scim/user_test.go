package scim

import (
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

func TestListUsersPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	// Create 5 users with names that will sort as: u-aaa, u-bbb, u-ccc, u-ddd, u-eee.
	users := []*v3.User{
		{ObjectMeta: metav1.ObjectMeta{Name: "u-ccc"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-aaa"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-eee"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-bbb"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-ddd"}},
	}

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return(users, nil).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		return &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"principalid": {provider + "_user://" + name},
					"username":    {"user-" + name},
				},
			},
		}, nil
	}).AnyTimes()

	srv := &SCIMServer{
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
		wantUserIDs      []string // Expected user IDs in order.
	}{
		{
			name:             "default pagination returns all users",
			queryString:      "",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 5,
			wantStartIndex:   1,
			wantUserIDs:      []string{"u-aaa", "u-bbb", "u-ccc", "u-ddd", "u-eee"},
		},
		{
			name:             "first page with count=2",
			queryString:      "count=2",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 2,
			wantStartIndex:   1,
			wantUserIDs:      []string{"u-aaa", "u-bbb"},
		},
		{
			name:             "second page with count=2",
			queryString:      "startIndex=3&count=2",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 2,
			wantStartIndex:   3,
			wantUserIDs:      []string{"u-ccc", "u-ddd"},
		},
		{
			name:             "last partial page",
			queryString:      "startIndex=5&count=2",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 1,
			wantStartIndex:   5,
			wantUserIDs:      []string{"u-eee"},
		},
		{
			name:             "startIndex beyond total returns empty",
			queryString:      "startIndex=100",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 0,
			wantStartIndex:   100,
			wantUserIDs:      []string{},
		},
		{
			name:             "count=0 returns empty resources with totalResults",
			queryString:      "count=0",
			wantStatus:       http.StatusOK,
			wantTotalResults: 5,
			wantItemsPerPage: 0,
			wantStartIndex:   1,
			wantUserIDs:      []string{},
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
		{
			name:        "invalid count returns error",
			queryString: "count=xyz",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Users?"+tt.queryString, nil)
			req = mux.SetURLVars(req, map[string]string{"provider": provider})
			rec := httptest.NewRecorder()

			srv.ListUsers(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusOK {
				var resp ListResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)

				assert.Equal(t, tt.wantTotalResults, resp.TotalResults)
				assert.Equal(t, tt.wantItemsPerPage, resp.ItemsPerPage)
				assert.Equal(t, tt.wantStartIndex, resp.StartIndex)
				assert.Len(t, resp.Resources, len(tt.wantUserIDs))

				// Verify the order of returned users.
				for i, wantID := range tt.wantUserIDs {
					resource := resp.Resources[i].(map[string]any)
					assert.Equal(t, wantID, resource["id"])
				}
			}
		})
	}
}

func TestListUsersPaginationConsistency(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	// Create 10 users.
	users := make([]*v3.User, 10)
	for i := 0; i < 10; i++ {
		users[i] = &v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("u-%03d", i)},
		}
	}

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return(users, nil).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		return &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"principalid": {provider + "_user://" + name},
					"username":    {"user-" + name},
				},
			},
		}, nil
	}).AnyTimes()

	srv := &SCIMServer{
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
	}

	// Collect all user IDs by paginating through all pages.
	pageSize := 3
	var allCollectedIDs []string

	for startIndex := 1; ; startIndex += pageSize {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1-scim/%s/Users?startIndex=%d&count=%d", provider, startIndex, pageSize), nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.ListUsers(rec, req)
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

	// Verify we collected all 10 users.
	assert.Len(t, allCollectedIDs, 10)

	// Verify no duplicates.
	seen := make(map[string]bool)
	for _, id := range allCollectedIDs {
		assert.False(t, seen[id], "Duplicate user ID found: %s", id)
		seen[id] = true
	}

	// Verify sorted order.
	for i := 1; i < len(allCollectedIDs); i++ {
		assert.True(t, allCollectedIDs[i-1] < allCollectedIDs[i],
			"Users not in sorted order: %s should come before %s", allCollectedIDs[i-1], allCollectedIDs[i])
	}
}

func TestListUsersWithFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	users := []*v3.User{
		{ObjectMeta: metav1.ObjectMeta{Name: "u-aaa"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-bbb"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-ccc"}},
	}

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return(users, nil).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		usernames := map[string]string{
			"u-aaa": "john.doe",
			"u-bbb": "jane.smith",
			"u-ccc": "bob.wilson",
		}
		return &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"principalid": {provider + "_user://" + name},
					"username":    {usernames[name]},
				},
			},
		}, nil
	}).AnyTimes()

	srv := &SCIMServer{
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
	}

	req := httptest.NewRequest(http.MethodGet, `/v1-scim/`+provider+`/Users?filter=userName%20eq%20%22jane.smith%22`, nil)
	req = mux.SetURLVars(req, map[string]string{"provider": provider})
	rec := httptest.NewRecorder()

	srv.ListUsers(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.TotalResults)
	assert.Equal(t, 1, resp.ItemsPerPage)
	require.Len(t, resp.Resources, 1)

	resource := resp.Resources[0].(map[string]any)
	assert.Equal(t, "u-bbb", resource["id"])
	assert.Equal(t, "jane.smith", resource["userName"])
}

func TestListUsersExcludesSystemUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := "okta"

	users := []*v3.User{
		{ObjectMeta: metav1.ObjectMeta{Name: "u-aaa"}},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-system"},
			PrincipalIDs: []string{"system://local"}, // System user.
		},
		{ObjectMeta: metav1.ObjectMeta{Name: "u-bbb"}},
	}

	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().List(labels.Everything()).Return(users, nil).AnyTimes()

	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		if name == "u-system" {
			return nil, apierrors.NewNotFound(v3.Resource("userattributes"), name)
		}
		return &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"principalid": {provider + "_user://" + name},
					"username":    {"user-" + name},
				},
			},
		}, nil
	}).AnyTimes()

	srv := &SCIMServer{
		userCache:          userCache,
		userAttributeCache: userAttributeCache,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Users", nil)
	req = mux.SetURLVars(req, map[string]string{"provider": provider})
	rec := httptest.NewRecorder()

	srv.ListUsers(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Should only have 2 users (system user excluded).
	assert.Equal(t, 2, resp.TotalResults)
	assert.Len(t, resp.Resources, 2)
}
