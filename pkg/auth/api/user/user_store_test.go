package user

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	types.Store
	updateData map[string]interface{}

	byIDResults []map[string]interface{}
	byIDErr     error
	byIDCalls   int
}

func (m *mockStore) Update(_ *types.APIContext, _ *types.Schema, data map[string]interface{}, _ string) (map[string]interface{}, error) {
	m.updateData = data
	return data, nil
}

func (m *mockStore) ByID(_ *types.APIContext, _ *types.Schema, _ string) (map[string]interface{}, error) {
	m.byIDCalls++
	if m.byIDErr != nil {
		return nil, m.byIDErr
	}
	if m.byIDCalls <= len(m.byIDResults) {
		return m.byIDResults[m.byIDCalls-1], nil
	}
	return m.byIDResults[len(m.byIDResults)-1], nil
}

func TestUpdateStripsIdentityFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		data        map[string]interface{}
		wantAbsent  []string
		wantPresent map[string]interface{}
	}{
		{
			name: "strips principalIds",
			data: map[string]interface{}{
				client.UserFieldPrincipalIDs: []interface{}{"local://u-abc", "github://12345"},
				client.UserFieldDescription:  "some user",
			},
			wantAbsent:  []string{client.UserFieldPrincipalIDs},
			wantPresent: map[string]interface{}{client.UserFieldDescription: "some user"},
		},
		{
			name: "strips username",
			data: map[string]interface{}{
				client.UserFieldUsername:    "admin",
				client.UserFieldDescription: "some user",
			},
			wantAbsent:  []string{client.UserFieldUsername},
			wantPresent: map[string]interface{}{client.UserFieldDescription: "some user"},
		},
		{
			name: "strips both identity fields",
			data: map[string]interface{}{
				client.UserFieldPrincipalIDs: []interface{}{"local://u-abc"},
				client.UserFieldUsername:     "admin",
				client.UserFieldEnabled:      true,
			},
			wantAbsent:  []string{client.UserFieldPrincipalIDs, client.UserFieldUsername},
			wantPresent: map[string]interface{}{client.UserFieldEnabled: true},
		},
		{
			name: "passes through non-identity fields",
			data: map[string]interface{}{
				client.UserFieldDescription: "updated description",
				client.UserFieldEnabled:     false,
			},
			wantPresent: map[string]interface{}{
				client.UserFieldDescription: "updated description",
				client.UserFieldEnabled:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inner := &mockStore{}
			s := &userStore{Store: inner}

			req := httptest.NewRequest("PUT", "/v3/users/u-abc", nil)
			req.Header.Set("Impersonate-User", "u-other")

			apiCtx := &types.APIContext{Request: req}

			_, err := s.Update(apiCtx, nil, tt.data, "u-abc")
			require.NoError(t, err)

			for _, key := range tt.wantAbsent {
				assert.NotContains(t, inner.updateData, key)
			}
			for key, val := range tt.wantPresent {
				assert.Equal(t, val, inner.updateData[key], "expected %q = %v", key, val)
			}
		})
	}
}

func TestWithPrincipalIDs(t *testing.T) {
	t.Parallel()

	created := map[string]interface{}{types.ResourceFieldID: "u-abc"}
	withIDs := map[string]interface{}{
		types.ResourceFieldID:        "u-abc",
		client.UserFieldPrincipalIDs: []interface{}{"local://u-abc"},
	}
	withoutIDs := map[string]interface{}{types.ResourceFieldID: "u-abc"}
	emptyIDs := map[string]interface{}{
		types.ResourceFieldID:        "u-abc",
		client.UserFieldPrincipalIDs: []interface{}{},
	}

	tests := []struct {
		name          string
		byIDResults   []map[string]interface{}
		byIDErr       error
		want          map[string]interface{}
		wantByIDCalls int
	}{
		{
			name:          "principal IDs available on the first attempt",
			byIDResults:   []map[string]interface{}{withIDs},
			want:          withIDs,
			wantByIDCalls: 1,
		},
		{
			name:          "principal IDs populated after a few attempts",
			byIDResults:   []map[string]interface{}{withoutIDs, emptyIDs, withIDs},
			want:          withIDs,
			wantByIDCalls: 3,
		},
		{
			name:          "returns the created user when principal IDs never show up",
			byIDResults:   []map[string]interface{}{withoutIDs},
			want:          created,
			wantByIDCalls: principalIDRetries,
		},
		{
			name:          "returns the created user when the lookup keeps failing",
			byIDErr:       fmt.Errorf("some error"),
			want:          created,
			wantByIDCalls: principalIDRetries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inner := &mockStore{byIDResults: tt.byIDResults, byIDErr: tt.byIDErr}
			s := &userStore{Store: inner, principalIDRetryDelay: time.Microsecond}

			got := s.withPrincipalIDs(nil, nil, created, "u-abc")

			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantByIDCalls, inner.byIDCalls)
		})
	}
}
