package common_test

import (
	"errors"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestUserSearcherSearchPrincipals(t *testing.T) {
	t.Parallel()

	users := []*apiv3.User{
		{
			// OIDC user: no Username, opaque subject in the provider principal.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc1"},
			DisplayName:  "Test UserOne",
			PrincipalIDs: []string{"genericoidc_user://sub-0001", "local://u-oidc1"},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc2"},
			DisplayName:  "Test UserTwo",
			PrincipalIDs: []string{"genericoidc_user://sub-0002", "local://u-oidc2"},
		},
		{
			// SCIM-provisioned Okta user.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-okta1"},
			DisplayName:  "testuser2@example.com",
			PrincipalIDs: []string{"okta_user://abc-uuid-1", "local://u-okta1"},
		},
		{
			// Purely local user, no external principal.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-local1"},
			Username:     "svcadmin",
			DisplayName:  "Service Admin",
			PrincipalIDs: []string{"local://u-local1"},
		},
		{
			// Hybrid user: local login plus an external principal.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-admin"},
			Username:     "admin",
			DisplayName:  "Default Admin",
			PrincipalIDs: []string{"genericoidc_user://sub-0009", "local://u-admin"},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-accent"},
			DisplayName:  "Émile Dubois",
			PrincipalIDs: []string{"genericoidc_user://sub-0003", "local://u-accent"},
		},
	}

	searcher := common.NewUserSearcher(&fakes.UserListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
			return users, nil
		},
	})

	tests := []struct {
		name      string
		provider  string
		searchKey string
		want      []apiv3.Principal
	}{
		{
			name:      "matches display name of a provider user",
			provider:  "genericoidc",
			searchKey: "testu",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0001"},
					DisplayName:   "Test UserOne",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0002"},
					DisplayName:   "Test UserTwo",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
			},
		},
		{
			name:      "match is case insensitive and ignores whitespace",
			provider:  "genericoidc",
			searchKey: "test user",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0001"},
					DisplayName:   "Test UserOne",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0002"},
					DisplayName:   "Test UserTwo",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
			},
		},
		{
			name:      "match ignores diacritics",
			provider:  "genericoidc",
			searchKey: "emile",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0003"},
					DisplayName:   "Émile Dubois",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
			},
		},
		{
			name:      "hybrid user is returned under the external provider",
			provider:  "genericoidc",
			searchKey: "Default Admin",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0009"},
					DisplayName:   "Default Admin",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
			},
		},
		{
			name:      "matches the local login username of a hybrid user",
			provider:  "genericoidc",
			searchKey: "admin",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0009"},
					DisplayName:   "Default Admin",
					PrincipalType: "user",
					Provider:      "genericoidc",
				},
			},
		},
		{
			name:      "only principals owned by the requested provider are returned",
			provider:  "okta",
			searchKey: "testuser2",
			want: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "okta_user://abc-uuid-1"},
					DisplayName:   "testuser2@example.com",
					PrincipalType: "user",
					Provider:      "okta",
				},
			},
		},
		{
			name:      "user without a principal for the provider is skipped",
			provider:  "genericoidc",
			searchKey: "svcadmin",
			want:      nil,
		},
		{
			name:      "no match returns nothing",
			provider:  "genericoidc",
			searchKey: "nobody",
			want:      nil,
		},
		{
			name:      "empty search key returns nothing",
			provider:  "genericoidc",
			searchKey: "",
			want:      nil,
		},
		{
			name:      "whitespace search key returns nothing",
			provider:  "genericoidc",
			searchKey: "   ",
			want:      nil,
		},
		{
			name:      "long whitespace search key returns nothing",
			provider:  "genericoidc",
			searchKey: "          ",
			want:      nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := searcher.SearchPrincipals(test.provider, test.searchKey)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestUserSearcherSearchPrincipalsOrder(t *testing.T) {
	t.Parallel()

	// The informer cache iterates in no particular order, so the lister can
	// hand back matching users in any order.
	users := []*apiv3.User{
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc3"},
			DisplayName:  "Test UserThree",
			PrincipalIDs: []string{"genericoidc_user://sub-0003"},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc1"},
			DisplayName:  "Test UserOne",
			PrincipalIDs: []string{"genericoidc_user://sub-0001"},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc2"},
			DisplayName:  "Test UserTwo",
			PrincipalIDs: []string{"genericoidc_user://sub-0002"},
		},
	}

	searcher := common.NewUserSearcher(&fakes.UserListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
			return users, nil
		},
	})

	got, err := searcher.SearchPrincipals("genericoidc", "testu")
	require.NoError(t, err)

	var ids []string
	for _, principal := range got {
		ids = append(ids, principal.Name)
	}

	assert.Equal(t, []string{
		"genericoidc_user://sub-0001",
		"genericoidc_user://sub-0002",
		"genericoidc_user://sub-0003",
	}, ids)
}

func TestUserSearcherSearchPrincipalsListError(t *testing.T) {
	t.Parallel()

	searcher := common.NewUserSearcher(&fakes.UserListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
			return nil, errors.New("cache is not synced")
		},
	})

	got, err := searcher.SearchPrincipals("genericoidc", "testu")
	require.ErrorContains(t, err, `listing users for search "testu": cache is not synced`)
	assert.Nil(t, got)
}
