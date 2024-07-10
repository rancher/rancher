package genericoidc

import (
	"reflect"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenOIDCProvider_GetPrincipal(t *testing.T) {
	tests := []struct {
		name        string
		principalID string
		token       v3.Token
		want        v3.Principal
		wantErr     bool
	}{
		{
			name:        "fetch principal for current user",
			principalID: "genericoidc_user://1234567",
			token: v3.Token{
				UserPrincipal: apimgmtv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "genericoidc_user://1234567",
					},
					DisplayName:   "Test User",
					LoginName:     "1234567",
					PrincipalType: "user",
					Me:            true,
				},
			},
			want: v3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://1234567",
				},
				DisplayName:   "Test User",
				LoginName:     "1234567",
				PrincipalType: "user",
				Me:            true,
				Provider:      Name,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal for user other than self",
			principalID: "genericoidc_user://9876543",
			token: v3.Token{
				UserPrincipal: apimgmtv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "genericoidc_user://1234567",
					},
					DisplayName:   "Test User",
					LoginName:     "1234567",
					PrincipalType: "user",
					Me:            false,
				},
			},
			want: v3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      Name,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal token is nil",
			principalID: "genericoidc_user://9876543",
			want: v3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      Name,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal called with empty principal",
			principalID: "",
			want:        v3.Principal{},
			wantErr:     true,
		},
	}
	for _, test := range tests {
		test := test
		g := &GenOIDCProvider{
			oidc.OpenIDCProvider{
				Name: Name,
				Type: client.GenericOIDCConfigType,
			},
		}
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := g.GetPrincipal(test.principalID, test.token)
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

func TestGenOIDCProvider_SearchPrincipals(t *testing.T) {
	tests := []struct {
		name          string
		searchValue   string
		principalType string
		expected      []v3.Principal
	}{
		{
			name:          "test search for user principal",
			searchValue:   "user1",
			principalType: UserType,
			expected: []v3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://user1"},
					DisplayName:   "user1",
					LoginName:     "user1",
					PrincipalType: UserType,
					Provider:      Name,
				},
			},
		},
		{
			name:        "test search for user principal with empty principaltype",
			searchValue: "user1",
			expected: []v3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://user1"},
					DisplayName:   "user1",
					LoginName:     "user1",
					PrincipalType: UserType,
					Provider:      Name,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://user1"},
					DisplayName:   "user1",
					PrincipalType: GroupType,
					Provider:      Name,
				},
			},
		},
		{
			name: "test search for user principal with empty principaltype and searchval",
			expected: []v3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_user://"},
					PrincipalType: UserType,
					Provider:      Name,
				},
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://"},
					PrincipalType: GroupType,
					Provider:      Name,
				},
			},
		},
		{
			name:          "test search for group principal",
			searchValue:   "group1",
			principalType: GroupType,
			expected: []v3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://group1"},
					DisplayName:   "group1",
					PrincipalType: GroupType,
					Provider:      Name,
				},
			},
		},
		{
			name:          "test search for group principal with empty searchval",
			principalType: GroupType,
			expected: []v3.Principal{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "genericoidc_group://"},
					PrincipalType: GroupType,
					Provider:      Name,
				},
			},
		},
	}

	g := &GenOIDCProvider{
		oidc.OpenIDCProvider{
			Name: Name,
			Type: client.GenericOIDCConfigType,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := g.SearchPrincipals(test.searchValue, test.principalType, v3.Token{})
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
		authConfig map[string]interface{}
		expected   map[string]interface{}
	}{
		{
			name: "Test with valid authConfig",
			authConfig: map[string]interface{}{
				"clientId":     "client123",
				"rancherUrl":   "https://example.com/callback",
				"scope":        "openid profile email",
				"issuer":       "https://ranchertest.io/issuer",
				"authEndpoint": "https://ranchertest.io/auth",
			},
			expected: map[string]interface{}{
				"redirectUrl": "https://ranchertest.io/auth?client_id=client123&response_type=code&redirect_uri=https://example.com/callback",
				"scopes":      "openid profile email",
			},
		},
	}

	provider := &GenOIDCProvider{}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := provider.TransformToAuthProvider(test.authConfig)
			if err != nil {
				t.Errorf("TransformToAuthProvider() returned an error: %v", err)
			}

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("TransformToAuthProvider() returned %+v, expected %+v", result, test.expected)
			}
		})
	}
}
