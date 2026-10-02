package genericoidc

import (
	"errors"
	"reflect"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestGenOIDCProvider_GetPrincipal(t *testing.T) {
	tests := []struct {
		name        string
		principalID string
		token       apiv3.Token
		want        apiv3.Principal
		wantErr     bool
	}{
		{
			name:        "fetch principal for current user",
			principalID: "genericoidc_user://1234567",
			token: apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "genericoidc_user://1234567",
					},
					DisplayName:   "Test User",
					LoginName:     "1234567",
					PrincipalType: "user",
					Me:            true,
				},
			},
			want: apiv3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://1234567",
				},
				DisplayName:   "Test User",
				LoginName:     "1234567",
				PrincipalType: "user",
				Me:            true,
				Provider:      ProviderName,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal for user other than self",
			principalID: "genericoidc_user://9876543",
			token: apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "genericoidc_user://1234567",
					},
					DisplayName:   "Test User",
					LoginName:     "1234567",
					PrincipalType: "user",
					Me:            false,
				},
			},
			want: apiv3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      ProviderName,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal token is nil",
			principalID: "genericoidc_user://9876543",
			want: apiv3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      ProviderName,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal called with empty principal",
			principalID: "",
			want:        apiv3.Principal{},
			wantErr:     true,
		},
	}
	for _, test := range tests {
		test := test
		g := &GenOIDCProvider{
			oidc.OpenIDCProvider{
				Name: ProviderName,
				Type: client.GenericOIDCConfigType,
			},
		}
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := g.GetPrincipal(test.principalID, &test.token)
			if (err != nil) != test.wantErr {
				t.Errorf("GetPrincipal() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("GetPrincipal() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGenOIDCProvider_GetPrincipalExt(t *testing.T) {
	tests := []struct {
		name        string
		principalID string
		token       ext.Token
		want        apiv3.Principal
		wantErr     bool
	}{
		// Note: ext tokens do not have `Me` information. current/other not distinguishable.
		{
			name:        "fetch principal",
			principalID: "genericoidc_user://1234567",
			token: ext.Token{
				Spec: ext.TokenSpec{
					UserPrincipal: ext.TokenPrincipal{
						Name:          "genericoidc_user://1234567",
						Provider:      ProviderName,
						DisplayName:   "Test User",
						LoginName:     "1234567",
						PrincipalType: "user",
					},
				},
			},
			want: apiv3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://1234567",
				},
				DisplayName:   "Test User",
				LoginName:     "1234567",
				PrincipalType: "user",
				Provider:      ProviderName,
				Me:            true,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal token is nil",
			principalID: "genericoidc_user://9876543",
			want: apiv3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      ProviderName,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal called with empty principal",
			principalID: "",
			want:        apiv3.Principal{},
			wantErr:     true,
		},
	}
	for _, test := range tests {
		test := test
		g := &GenOIDCProvider{
			oidc.OpenIDCProvider{
				Name: ProviderName,
				Type: client.GenericOIDCConfigType,
			},
		}
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := g.GetPrincipal(test.principalID, &test.token)
			assert.Equal(t, test.wantErr, err != nil)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestGenOIDCProvider_SearchPrincipals(t *testing.T) {
	tests := []struct {
		name          string
		searchValue   string
		principalType string
		expected      []apiv3.Principal
	}{
		{
			name:          "test search for user principal",
			searchValue:   "user1",
			principalType: UserType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://user1"},
					DisplayName:   "user1",
					LoginName:     "user1",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:        "test search for user principal with empty principaltype",
			searchValue: "user1",
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://user1"},
					DisplayName:   "user1",
					LoginName:     "user1",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://user1"},
					DisplayName:   "user1",
					PrincipalType: GroupType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name: "test search for user principal with empty principaltype and searchval",
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://"},
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://"},
					PrincipalType: GroupType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:          "test search for group principal",
			searchValue:   "group1",
			principalType: GroupType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://group1"},
					DisplayName:   "group1",
					PrincipalType: GroupType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:          "test search for group principal with empty searchval",
			principalType: GroupType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://"},
					PrincipalType: GroupType,
					Provider:      ProviderName,
				},
			},
		},
	}

	g := &GenOIDCProvider{
		oidc.OpenIDCProvider{
			Name: ProviderName,
			Type: client.GenericOIDCConfigType,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			token := &apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{Name: "genericoidc_user://testuser"},
				},
			}
			result, err := g.SearchPrincipals(test.searchValue, test.principalType, token)
			if err != nil {
				t.Errorf("SearchPrincipals() returned an error: %v", err)
			}

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("SearchPrincipals() returned %+v, expected %+v", result, test.expected)
			}
		})

		// And same behaviour for ext tokens
		t.Run(test.name+", ext", func(t *testing.T) {
			t.Parallel()
			token := &ext.Token{
				Spec: ext.TokenSpec{
					UserPrincipal: ext.TokenPrincipal{
						Name: "genericoidc_user://testuser",
					},
				},
			}
			result, err := g.SearchPrincipals(test.searchValue, test.principalType, token)
			if err != nil {
				t.Errorf("SearchPrincipals() returned an error: %v", err)
			}

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("SearchPrincipals() returned %+v, expected %+v", result, test.expected)
			}
		})
	}
}

func TestGenOIDCProvider_TransformToAuthProvider(t *testing.T) {
	tests := []struct {
		name       string
		authConfig map[string]any
		expected   map[string]any
	}{
		{
			name: "Test with valid authConfig",
			authConfig: map[string]any{
				"metadata":     map[string]any{"name": "genericoidc"},
				"clientId":     "client123",
				"rancherUrl":   "https://example.com/callback",
				"scope":        "openid profile email",
				"issuer":       "https://ranchertest.io/issuer",
				"authEndpoint": "https://ranchertest.io/auth",
			},
			expected: map[string]any{
				"id":                 "genericoidc",
				"redirectUrl":        "https://ranchertest.io/auth?client_id=client123&response_type=code&redirect_uri=https://example.com/callback",
				"scopes":             "openid profile email",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			name: "When configuration has acrValue",
			authConfig: map[string]any{
				"metadata":     map[string]any{"name": "genericoidc"},
				"clientId":     "client123",
				"rancherUrl":   "https://example.com/callback",
				"scope":        "openid profile email",
				"issuer":       "https://ranchertest.io/issuer",
				"authEndpoint": "https://ranchertest.io/auth",
				"acrValue":     "testing",
			},
			expected: map[string]any{
				"id":                 "genericoidc",
				"redirectUrl":        "https://ranchertest.io/auth?client_id=client123&response_type=code&redirect_uri=https://example.com/callback&acr_values=testing",
				"scopes":             "openid profile email",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
	}

	provider := &GenOIDCProvider{
		baseoidc.OpenIDCProvider{},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := provider.TransformToAuthProvider(test.authConfig)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestGenOIDCProviderSearchPrincipalsResolvesKnownUsers(t *testing.T) {
	t.Parallel()

	users := []*apiv3.User{
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-oidc1"},
			DisplayName:  "Test UserOne",
			PrincipalIDs: []string{"genericoidc_user://sub-0001", "local://u-oidc1"},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-okta1"},
			DisplayName:  "Test UserFromOkta",
			PrincipalIDs: []string{"okta_user://abc-uuid-1", "local://u-okta1"},
		},
	}

	g := &GenOIDCProvider{
		oidc.OpenIDCProvider{
			Name: ProviderName,
			Type: client.GenericOIDCConfigType,
			UserSearcher: common.NewUserSearcher(&fakes.UserListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
					return users, nil
				},
			}),
		},
	}

	tests := []struct {
		name          string
		searchValue   string
		principalType string
		expected      []apiv3.Principal
	}{
		{
			name:          "known user is returned with its real subject",
			searchValue:   "testu",
			principalType: UserType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0001"},
					DisplayName:   "Test UserOne",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://testu"},
					DisplayName:   "testu",
					LoginName:     "testu",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:        "known user is returned alongside the group principal",
			searchValue: "testu",
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0001"},
					DisplayName:   "Test UserOne",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://testu"},
					DisplayName:   "testu",
					LoginName:     "testu",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://testu"},
					DisplayName:   "testu",
					PrincipalType: GroupType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:          "searching the subject returns a single principal",
			searchValue:   "sub-0001",
			principalType: UserType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://sub-0001"},
					DisplayName:   "sub-0001",
					LoginName:     "sub-0001",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:          "users of another provider are not returned",
			searchValue:   "UserFromOkta",
			principalType: UserType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://UserFromOkta"},
					DisplayName:   "UserFromOkta",
					LoginName:     "UserFromOkta",
					PrincipalType: UserType,
					Provider:      ProviderName,
				},
			},
		},
		{
			name:          "group search does not resolve users",
			searchValue:   "testu",
			principalType: GroupType,
			expected: []apiv3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://testu"},
					DisplayName:   "testu",
					PrincipalType: GroupType,
					Provider:      ProviderName,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token := &apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{Name: "genericoidc_user://testuser"},
				},
			}
			result, err := g.SearchPrincipals(test.searchValue, test.principalType, token)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestGenOIDCProviderSearchPrincipalsUserSearchError(t *testing.T) {
	t.Parallel()

	g := &GenOIDCProvider{
		oidc.OpenIDCProvider{
			Name: ProviderName,
			Type: client.GenericOIDCConfigType,
			UserSearcher: common.NewUserSearcher(&fakes.UserListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*apiv3.User, error) {
					return nil, errors.New("cache is not synced")
				},
			}),
		},
	}

	token := &apiv3.Token{
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: "genericoidc_user://testuser"},
		},
	}
	got, err := g.SearchPrincipals("testu", UserType, token)
	assert.ErrorContains(t, err, "cache is not synced")
	assert.Nil(t, got)
}
