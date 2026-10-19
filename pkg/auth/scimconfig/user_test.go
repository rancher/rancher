package scimconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserIdentity(t *testing.T) {
	t.Parallel()

	userNameMode := Default()
	externalIDMode := Config{UserIDAttribute: UserIDExternalID}

	tests := []struct {
		name           string
		cfg            Config
		extras         map[string][]string
		wantUserName   string
		wantExternalID string
	}{
		{
			name: "userName mode: SCIM created, never logged in",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {"okta_user://alice"},
				"username":    {"alice"},
				"externalid":  {"ext-1"},
				"email":       {"alice@example.com"},
			},
			wantUserName:   "alice",
			wantExternalID: "ext-1",
		},
		{
			name: "userName mode: SCIM created, logged in before the login kept SCIM data",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {"okta_user://CN=alice,DC=example,DC=com"},
				"username":    {"alice"},
			},
			wantUserName:   "CN=alice,DC=example,DC=com",
			wantExternalID: "",
		},
		{
			name: "userName mode: SCIM created, logged in after the login kept SCIM data",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {"okta_user://CN=alice,DC=example,DC=com"},
				"username":    {"alice"},
				"externalid":  {"ext-1"},
				"email":       {""},
			},
			wantUserName:   "CN=alice,DC=example,DC=com",
			wantExternalID: "ext-1",
		},
		{
			name: "userName mode: login only",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {"okta_user://bob"},
				"username":    {"Bob Login"},
			},
			wantUserName:   "bob",
			wantExternalID: "",
		},
		{
			name: "userName mode: written after the change",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid":  {"okta_user://alice"},
				"scimusername": {"alice"},
				"username":     {"alice.login"},
				"externalid":   {"ext-1"},
			},
			wantUserName:   "alice",
			wantExternalID: "ext-1",
		},
		{
			name: "userName mode: stored scimusername comes before the principal",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid":  {"okta_user://other"},
				"scimusername": {"alice"},
				"username":     {"alice.login"},
			},
			wantUserName: "alice",
		},
		{
			name: "userName mode: principal with another prefix",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {"github_user://alice"},
				"username":    {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "userName mode: principal of a provider whose name starts with the provider",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {"oktapreview_user://alice"},
				"username":    {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "userName mode: missing principalid",
			cfg:  userNameMode,
			extras: map[string][]string{
				"username": {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "userName mode: empty principalid",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {},
				"username":    {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "userName mode: empty principalid value",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid": {""},
				"username":    {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "userName mode: empty scimusername value falls back to the principal",
			cfg:  userNameMode,
			extras: map[string][]string{
				"principalid":  {"okta_user://alice"},
				"scimusername": {""},
			},
			wantUserName: "alice",
		},
		{
			name:   "userName mode: no extras",
			cfg:    userNameMode,
			extras: nil,
		},
		{
			name: "externalId mode: SCIM created, never logged in",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"okta_user://ext-1"},
				"username":    {"alice"},
				"externalid":  {"ext-1"},
				"email":       {"alice@example.com"},
			},
			wantUserName:   "alice",
			wantExternalID: "ext-1",
		},
		{
			name: "externalId mode: SCIM created, logged in before the login kept SCIM data",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"okta_user://ext-1"},
				"username":    {"alice.login"},
			},
			wantUserName:   "alice.login",
			wantExternalID: "ext-1",
		},
		{
			name: "externalId mode: SCIM created, logged in after the login kept SCIM data",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"okta_user://ext-1"},
				"username":    {"alice.login"},
				"externalid":  {"ext-1"},
				"email":       {""},
			},
			wantUserName:   "alice.login",
			wantExternalID: "ext-1",
		},
		{
			name: "externalId mode: login only",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"okta_user://ext-2"},
				"username":    {"bob.login"},
			},
			wantUserName:   "bob.login",
			wantExternalID: "ext-2",
		},
		{
			name: "externalId mode: written after the change",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid":  {"okta_user://ext-1"},
				"scimusername": {"alice"},
				"username":     {"alice.login"},
				"externalid":   {"ext-1"},
			},
			wantUserName:   "alice",
			wantExternalID: "ext-1",
		},
		{
			name: "externalId mode: stored externalid comes before the principal",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"okta_user://other"},
				"externalid":  {"ext-1"},
			},
			wantExternalID: "ext-1",
		},
		{
			name: "externalId mode: empty externalid value falls back to the principal",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"okta_user://ext-1"},
				"externalid":  {""},
			},
			wantExternalID: "ext-1",
		},
		{
			name: "externalId mode: principal with another prefix",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {"github_user://ext-1"},
				"username":    {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "externalId mode: missing principalid",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"username": {"alice.login"},
			},
			wantUserName: "alice.login",
		},
		{
			name: "externalId mode: empty principalid value",
			cfg:  externalIDMode,
			extras: map[string][]string{
				"principalid": {""},
			},
		},
		{
			name:   "externalId mode: no extras",
			cfg:    externalIDMode,
			extras: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userName, externalID := UserIdentity(tt.extras, "okta", tt.cfg)
			assert.Equal(t, tt.wantUserName, userName)
			assert.Equal(t, tt.wantExternalID, externalID)
		})
	}
}
