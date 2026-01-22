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
	"github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestBoolUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{name: "true as boolean", input: "true", want: true},
		{name: "false as boolean", input: "false", want: false},
		{name: "true as string", input: `"true"`, want: true},
		{name: "false as string", input: `"false"`, want: false},
		{name: "invalid string", input: `"invalid"`, wantErr: true},
		{name: "number is invalid", input: "1", wantErr: true},
		{name: "null is invalid", input: "null", wantErr: true},
		{name: "empty string is invalid", input: `""`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Bool
			err := json.Unmarshal([]byte(tt.input), &b)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid boolean value")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, bool(b))
			}
		})
	}
}

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

func TestGetUser(t *testing.T) {
	provider := "okta"

	t.Run("user found with all attributes", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		enabled := true
		userID := "u-abc123"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":    {"john.doe"},
					"externalid":  {"ext-12345"},
					"principalid": {provider + "_user://john.doe"},
					"email":       {"john.doe@example.com"},
				},
			},
		}, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.GetUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, userID, resp["id"])
		assert.Equal(t, "john.doe", resp["userName"])
		assert.Equal(t, "ext-12345", resp["externalId"])
		assert.Equal(t, true, resp["active"])
		assert.Equal(t, []any{UserSchemaID}, resp["schemas"])

		meta, ok := resp["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, UserResource, meta["resourceType"])
		assert.Contains(t, meta["location"], "/v1-scim/"+provider+"/Users/"+userID)

		emails, ok := resp["emails"].([]any)
		require.True(t, ok)
		require.Len(t, emails, 1)
		email := emails[0].(map[string]any)
		assert.Equal(t, "john.doe@example.com", email["value"])
		assert.Equal(t, true, email["primary"])
	})

	t.Run("disabled user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		disabled := false
		userID := "u-disabled"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &disabled,
		}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":    {"inactive.user"},
					"externalid":  {"ext-inactive"},
					"principalid": {provider + "_user://inactive.user"},
				},
			},
		}, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.GetUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, userID, resp["id"])
		assert.Equal(t, "inactive.user", resp["userName"])
		assert.Equal(t, false, resp["active"])
	})

	t.Run("user not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-notfound"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(nil, apierrors.NewNotFound(v3.Resource("user"), userID))

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.GetUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})

	t.Run("system user returns not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-system"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: userID},
			PrincipalIDs: []string{"system://local"},
		}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.GetUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})
}

func TestCreateUser(t *testing.T) {
	provider := "okta"

	t.Run("creates user successfully with all fields", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		enabled := true
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureUser("okta_user://john.doe", "john.doe").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-abc123"},
			Enabled:    &enabled,
		}, nil)
		userMGR.EXPECT().UserAttributeCreateOrUpdate(
			"u-abc123",
			provider,
			[]v3.Principal{},
			map[string][]string{
				"username":    {"john.doe"},
				"externalid":  {"ext-12345"},
				"principalid": {"okta_user://john.doe"},
				"email":       {"john.doe@example.com"},
			},
		).Return(nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"externalId": "ext-12345",
			"emails": [{"value": "john.doe@example.com", "primary": true}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "u-abc123", resp["id"])
		assert.Equal(t, "john.doe", resp["userName"])
		assert.Equal(t, "ext-12345", resp["externalId"])
		assert.Equal(t, true, resp["active"])
		assert.Equal(t, []any{UserSchemaID}, resp["schemas"])

		emails, ok := resp["emails"].([]any)
		require.True(t, ok)
		require.Len(t, emails, 1)
		email := emails[0].(map[string]any)
		assert.Equal(t, "john.doe@example.com", email["value"])
		assert.Equal(t, true, email["primary"])
	})

	t.Run("creates user without email", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		enabled := true
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureUser("okta_user://jane.doe", "jane.doe").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-def456"},
			Enabled:    &enabled,
		}, nil)
		userMGR.EXPECT().UserAttributeCreateOrUpdate(
			"u-def456",
			provider,
			[]v3.Principal{},
			map[string][]string{
				"username":    {"jane.doe"},
				"externalid":  {"ext-67890"},
				"principalid": {"okta_user://jane.doe"},
			},
		).Return(nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "jane.doe",
			"externalId": "ext-67890"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "u-def456", resp["id"])
		assert.Equal(t, "jane.doe", resp["userName"])
		assert.Nil(t, resp["emails"])
	})

	t.Run("conflict when username already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingUser := &v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-existing"},
		}

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{existingUser}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-existing").Return(&v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "u-existing"},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username": {"john.doe"},
				},
			},
		}, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "John.Doe",
			"externalId": "ext-12345"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})

	t.Run("invalid request body", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `not valid json`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})

	t.Run("error listing users", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("cache error"))

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error ensuring user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureUser("okta_user://john.doe", "john.doe").Return(nil, fmt.Errorf("failed to create user"))

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error creating user attributes", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)

		enabled := true
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureUser("okta_user://john.doe", "john.doe").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-abc123"},
			Enabled:    &enabled,
		}, nil)
		userMGR.EXPECT().UserAttributeCreateOrUpdate(
			"u-abc123",
			provider,
			gomock.Any(),
			gomock.Any(),
		).Return(fmt.Errorf("failed to create attributes"))

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("skips users without attributes during duplicate check", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		existingUser := &v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-noattr"},
		}

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().List(labels.Everything()).Return([]*v3.User{existingUser}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get("u-noattr").Return(nil, apierrors.NewNotFound(v3.Resource("userattribute"), "u-noattr"))

		enabled := true
		userMGR := mocks.NewMockManager(ctrl)
		userMGR.EXPECT().EnsureUser("okta_user://john.doe", "john.doe").Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: "u-abc123"},
			Enabled:    &enabled,
		}, nil)
		userMGR.EXPECT().UserAttributeCreateOrUpdate(
			"u-abc123",
			provider,
			gomock.Any(),
			gomock.Any(),
		).Return(nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userMGR:            userMGR,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe"
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1-scim/"+provider+"/Users", bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider})
		rec := httptest.NewRecorder()

		srv.CreateUser(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
	})
}

func TestUpdateUser(t *testing.T) {
	provider := "okta"

	t.Run("updates userName successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":    {"old.name"},
					"externalid":  {"ext-12345"},
					"principalid": {provider + "_user://old.name"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			assert.Equal(t, "new.name", first(attr.ExtraByProvider[provider]["username"]))
			return attr, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "new.name",
			"externalId": "ext-12345",
			"active": true
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, userID, resp["id"])
		assert.Equal(t, "new.name", resp["userName"])
		assert.Equal(t, true, resp["active"])
	})

	t.Run("deactivates user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(u *v3.User) (*v3.User, error) {
			assert.Equal(t, false, *u.Enabled)
			return u, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"externalId": "ext-12345",
			"active": false
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, false, resp["active"])
	})

	t.Run("reactivates user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		disabled := false

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &disabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(u *v3.User) (*v3.User, error) {
			assert.Equal(t, true, *u.Enabled)
			return u, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"externalId": "ext-12345",
			"active": true
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, true, resp["active"])
	})

	t.Run("no update when nothing changed", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		// No Update calls expected since nothing changed.
		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"externalId": "ext-12345",
			"active": true
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("user not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-notfound"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(nil, apierrors.NewNotFound(v3.Resource("user"), userID))

		srv := &SCIMServer{
			userCache: userCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"active": true
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("system user returns not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-system"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: userID},
			PrincipalIDs: []string{"system://local"},
		}, nil)

		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(&v3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{},
		}, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"active": true
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("cannot deactivate default admin", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-admin"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Username:   "admin",
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"admin"},
					"externalid": {"ext-admin"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "admin",
			"externalId": "ext-admin",
			"active": false
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})

	t.Run("invalid request body", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)

		srv := &SCIMServer{
			userCache: userCache,
		}

		body := `not valid json`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("error updating user attributes", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"old.name"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("update failed"))

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "new.name",
			"externalId": "ext-12345",
			"active": true
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error updating user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("update failed"))

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
			"userName": "john.doe",
			"externalId": "ext-12345",
			"active": false
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.UpdateUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestDeleteUser(t *testing.T) {
	provider := "okta"

	t.Run("deletes user successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
		}, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Delete(userID, gomock.Any()).Return(nil)

		srv := &SCIMServer{
			userCache: userCache,
			users:     userClient,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.DeleteUser(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("user not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-notfound"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(nil, apierrors.NewNotFound(v3.Resource("user"), userID))

		srv := &SCIMServer{
			userCache: userCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.DeleteUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})

	t.Run("system user returns not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-system"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: userID},
			PrincipalIDs: []string{"system://local"},
		}, nil)

		srv := &SCIMServer{
			userCache: userCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.DeleteUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("cannot delete default admin", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-admin"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Username:   "admin",
		}, nil)

		srv := &SCIMServer{
			userCache: userCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.DeleteUser(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)

		var errResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Contains(t, errResp["schemas"], ErrorSchemaID)
	})

	t.Run("error deleting user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
		}, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Delete(userID, gomock.Any()).Return(fmt.Errorf("delete failed"))

		srv := &SCIMServer{
			userCache: userCache,
			users:     userClient,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.DeleteUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error getting user from cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(nil, fmt.Errorf("cache error"))

		srv := &SCIMServer{
			userCache: userCache,
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1-scim/"+provider+"/Users/"+userID, nil)
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.DeleteUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPatchUser(t *testing.T) {
	provider := "okta"

	t.Run("replace active to false deactivates user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(u *v3.User) (*v3.User, error) {
			assert.Equal(t, false, *u.Enabled)
			return u, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": false}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["active"])
	})

	t.Run("replace active to true reactivates user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		disabled := false

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &disabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(u *v3.User) (*v3.User, error) {
			assert.Equal(t, true, *u.Enabled)
			return u, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": true}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, true, resp["active"])
	})

	t.Run("replace externalId", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"old-ext-id"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			assert.Equal(t, "new-ext-id", first(attr.ExtraByProvider[provider]["externalid"]))
			return attr, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "externalId", "value": "new-ext-id"}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new-ext-id", resp["externalId"])
	})

	t.Run("replace primary email", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
					"email":      {"old@example.com"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(attr *v3.UserAttribute) (*v3.UserAttribute, error) {
			assert.Equal(t, "new@example.com", first(attr.ExtraByProvider[provider]["email"]))
			return attr, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "emails[primary eq true].value", "value": "new@example.com"}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		emails, ok := resp["emails"].([]any)
		require.True(t, ok)
		require.Len(t, emails, 1)
		email := emails[0].(map[string]any)
		assert.Equal(t, "new@example.com", email["value"])
	})

	t.Run("bulk replace multiple fields", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"old-ext-id"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(u *v3.User) (*v3.User, error) {
			return u, nil
		})

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "value": {"externalId": "new-ext-id", "active": false}}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("no update when value unchanged", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"ext-12345"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		// No Update calls expected since nothing changed.
		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": true}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("user not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-notfound"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(nil, apierrors.NewNotFound(v3.Resource("user"), userID))

		srv := &SCIMServer{
			userCache: userCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": false}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("system user returns not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-system"

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: userID},
			PrincipalIDs: []string{"system://local"},
		}, nil)

		srv := &SCIMServer{
			userCache: userCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": false}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("cannot deactivate default admin", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-admin"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Username:   "admin",
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username": {"admin"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": false}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unsupported operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username": {"john.doe"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "add", "path": "active", "value": false}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unsupported path", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username": {"john.doe"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "unsupportedPath", "value": "test"}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid request body", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		srv := &SCIMServer{
			userCache: userCache,
		}

		body := `not valid json`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid value type for active", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username": {"john.doe"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": "not-a-boolean"}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("error updating user attributes", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username":   {"john.doe"},
					"externalid": {"old-ext-id"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userAttrClient := fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
		userAttrClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("update failed"))

		srv := &SCIMServer{
			userCache:          userCache,
			userAttributeCache: userAttributeCache,
			userAttributes:     userAttrClient,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "externalId", "value": "new-ext-id"}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("error updating user", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		userID := "u-abc123"
		enabled := true

		userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userCache.EXPECT().Get(userID).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			Enabled:    &enabled,
		}, nil)

		existingAttr := &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: userID},
			ExtraByProvider: map[string]map[string][]string{
				provider: {
					"username": {"john.doe"},
				},
			},
		}
		userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
		userAttributeCache.EXPECT().Get(userID).Return(existingAttr, nil)

		userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
		userClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("update failed"))

		srv := &SCIMServer{
			userCache:          userCache,
			users:              userClient,
			userAttributeCache: userAttributeCache,
		}

		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "active", "value": false}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/v1-scim/"+provider+"/Users/"+userID, bytes.NewBufferString(body))
		req = mux.SetURLVars(req, map[string]string{"provider": provider, "id": userID})
		rec := httptest.NewRecorder()

		srv.PatchUser(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
