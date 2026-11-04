package common

import (
	"strings"
	"testing"

	"github.com/rancher/norman/httperror"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSplitPrincipalID(t *testing.T) {
	tests := []struct {
		name              string
		principalID       string
		wantProviderType  string
		wantExternalID    string
		wantPrincipalType string
		wantErr           bool
	}{
		{
			name:              "valid user principal id",
			principalID:       "github_user://9253000",
			wantProviderType:  "github",
			wantExternalID:    "9253000",
			wantPrincipalType: "user",
		},
		{
			name:              "valid team principal id",
			principalID:       "github_team://9933605",
			wantProviderType:  "github",
			wantExternalID:    "9933605",
			wantPrincipalType: "team",
		},
		{
			name:              "valid principal id without double slash",
			principalID:       "github_org:9343010",
			wantProviderType:  "github",
			wantExternalID:    "9343010",
			wantPrincipalType: "org",
		},
		{
			name:        "invalid principal id missing colon",
			principalID: "github_user//9253000",
			wantErr:     true,
		},
		{
			name:        "invalid principal id missing underscore",
			principalID: "github://9253000",
			wantErr:     true,
		},
		{
			name:        "invalid principal id empty config name",
			principalID: "_user://9253000",
			wantErr:     true,
		},
		{
			name:              "empty external id is accepted",
			principalID:       "github_user:",
			wantProviderType:  "github",
			wantExternalID:    "",
			wantPrincipalType: "user",
		},
		{
			name:        "invalid principal id of a config named like local missing underscore",
			principalID: "localfoo://9253000",
			wantErr:     true,
		},
		{
			name:              "principal id of a config named like local",
			principalID:       "localsaml_user://9253000",
			wantProviderType:  "localsaml",
			wantExternalID:    "9253000",
			wantPrincipalType: "user",
		},
		{
			name:              "local principal id",
			principalID:       "local://u-7xtmvv4ohf",
			wantProviderType:  "local",
			wantExternalID:    "u-7xtmvv4ohf",
			wantPrincipalType: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProviderType, gotPrincipalType, gotExternalID, err := SplitPrincipalID(tt.principalID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for principalID %q, got nil", tt.principalID)
				}
				if !strings.Contains(err.Error(), "invalid principal id") {
					t.Fatalf("expected invalid principal id error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error for principalID %q, got %v", tt.principalID, err)
			}

			if gotProviderType != tt.wantProviderType {
				t.Fatalf("expected providerType %q, got %q", tt.wantProviderType, gotProviderType)
			}

			if gotExternalID != tt.wantExternalID {
				t.Fatalf("expected externalID %q, got %q", tt.wantExternalID, gotExternalID)
			}

			if gotPrincipalType != tt.wantPrincipalType {
				t.Fatalf("expected principalType %q, got %q", tt.wantPrincipalType, gotPrincipalType)
			}
		})
	}
}

func TestConfigNameFromToken(t *testing.T) {
	tests := []struct {
		name           string
		principalName  string
		wantConfigName string
		wantErr        bool
	}{
		{
			name:           "github user token",
			principalName:  "github_user://9253000",
			wantConfigName: "github",
		},
		{
			name:           "local user token",
			principalName:  "local_user://admin",
			wantConfigName: "local",
		},
		{
			name:          "invalid principal missing colon",
			principalName: "github_user//9253000",
			wantErr:       true,
		},
		{
			name:          "invalid principal missing underscore",
			principalName: "github://9253000",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: tt.principalName,
					},
					LoginName:     "developer",
					PrincipalType: "user",
				},
			}

			gotConfigName, err := ConfigNameFromToken(token)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for principal %q, got nil", tt.principalName)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error for principal %q, got %v", tt.principalName, err)
			}

			if gotConfigName != tt.wantConfigName {
				t.Fatalf("expected config name %q, got %q", tt.wantConfigName, gotConfigName)
			}
		})
	}
}

func TestConfigNameFromPrincipal(t *testing.T) {
	tests := map[string]struct {
		principal apiv3.Principal
		want      string
	}{
		"default config": {
			principal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github_user://1"}, Provider: "github"},
			want:      "github",
		},
		"named config": {
			principal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}, Provider: "github"},
			want:      "github-eu",
		},
		"local": {
			principal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "local://u-abc"}, Provider: "local"},
			want:      "local",
		},
		"unparseable falls back to provider": {
			principal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "garbage"}, Provider: "github"},
			want:      "github",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, ConfigNameFromPrincipal(tt.principal))
		})
	}
}

func TestConfigNameOrLocalFromToken(t *testing.T) {
	tests := map[string]struct {
		token   *apiv3.Token
		want    string
		wantErr string
	}{
		"default config": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github_user://1"}, Provider: "github"},
			},
			want: "github",
		},
		"named config": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}, Provider: "github"},
			},
			want: "github-eu",
		},
		"local": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "local://u-abc"}, Provider: "local"},
			},
			want: "local",
		},
		"no principal falls back to local, not the auth provider": {
			token: &apiv3.Token{AuthProvider: "github"},
			want:  "local",
		},
		"unparseable principal is an error": {
			token: &apiv3.Token{
				AuthProvider:  "github",
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "garbage"}, Provider: "github"},
			},
			wantErr: "invalid principal: garbage",
		},
		"principal with an empty config name is an error": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "_user://1"}},
			},
			wantErr: "invalid principal: _user://1",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ConfigNameOrLocalFromToken(tt.token)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTokenHasAuthConfig(t *testing.T) {
	tests := map[string]struct {
		token *apiv3.Token
		want  bool
	}{
		"external principal": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github-eu_user://1"}},
			},
			want: true,
		},
		"local principal": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "local://u-abc"}},
			},
			want: true,
		},
		"unparseable principal": {
			token: &apiv3.Token{
				UserPrincipal: apiv3.Principal{ObjectMeta: metav1.ObjectMeta{Name: "github://1"}},
			},
			want: true,
		},
		"no principal": {
			// The deprecated auth provider is ignored.
			token: &apiv3.Token{AuthProvider: "local"},
			want:  false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, TokenHasAuthConfig(tt.token))
		})
	}
}

func TestConfigNameForPrincipalLookup(t *testing.T) {
	tests := []struct {
		name            string
		principalID     string
		tokenPrincipal  string
		wantConfigName  string
		wantNotFound    bool
		wantErrContains string
	}{
		{
			name:           "principal from the token's config",
			principalID:    "openldap-it_group://cn=admins,dc=example,dc=com",
			tokenPrincipal: "openldap-it_user://cn=alice,dc=example,dc=com",
			wantConfigName: "openldap-it",
		},
		{
			name:           "principal from another config",
			principalID:    "openldap-it_user://cn=bob,dc=example,dc=com",
			tokenPrincipal: "openldap-corp_user://cn=alice,dc=example,dc=com",
			wantNotFound:   true,
		},
		{
			name:           "principal from the default config with a token from an additional config",
			principalID:    "openldap_user://cn=bob,dc=example,dc=com",
			tokenPrincipal: "openldap-corp_user://cn=alice,dc=example,dc=com",
			wantNotFound:   true,
		},
		{
			name:            "invalid principal ID",
			principalID:     "openldap-corp",
			tokenPrincipal:  "openldap-corp_user://cn=alice,dc=example,dc=com",
			wantErrContains: "invalid principal id openldap-corp",
		},
		{
			name:            "invalid token principal",
			principalID:     "openldap-corp_user://cn=bob,dc=example,dc=com",
			tokenPrincipal:  "openldap-corp",
			wantErrContains: "invalid principal: openldap-corp",
		},
		{
			// A fallback to "local" for an unparseable token principal would
			// make this lookup succeed, so it must fail with the parse error.
			name:            "invalid token principal with a local principal ID",
			principalID:     "local://u-1",
			tokenPrincipal:  "garbage",
			wantErrContains: "invalid principal: garbage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{Name: tt.tokenPrincipal},
				},
			}

			gotConfigName, err := ConfigNameForPrincipalLookup(tt.principalID, token)
			if tt.wantNotFound {
				assert.True(t, httperror.IsNotFound(err), "expected NotFound, got %v", err)
				return
			}
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				assert.False(t, httperror.IsNotFound(err), "expected a non-NotFound error, got %v", err)
				assert.Empty(t, gotConfigName)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantConfigName, gotConfigName)
		})
	}
}
