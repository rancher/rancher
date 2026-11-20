package v3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUserIsSystem(t *testing.T) {
	tests := []struct {
		user     *User
		isSystem bool
	}{
		{
			user: &User{
				PrincipalIDs: []string{"system://local", "local://u-b4qkhsnliz"},
			},
			isSystem: true,
		},
		{
			user: &User{
				PrincipalIDs: []string{"system://provisioning/fleet-local/local", "local://u-mo773yttt4"},
			},
			isSystem: true,
		},
		{
			user: &User{
				PrincipalIDs: []string{"local://u-cx7gc"},
			},
		},
		{
			user: &User{
				PrincipalIDs: []string{"activedirectory_user://CN=foo,CN=Users,DC=bar,DC=rancher,DC=space", "local://u-ckrl4grxg5"},
			},
		},
		{
			user: &User{},
		},
	}

	for _, tt := range tests {
		if want, got := tt.isSystem, tt.user.IsSystem(); want != got {
			t.Errorf("Expected %t got %t", want, got)
		}
	}
}

func TestUserIsAdmin(t *testing.T) {
	tests := []struct {
		user    *User
		isAdmin bool
	}{
		{
			user: &User{
				Username: "admin",
			},
			isAdmin: true,
		},
		{
			user: &User{
				Username: "u-ckrl4grxg5",
			},
		},
	}

	for _, tt := range tests {
		if want, got := tt.isAdmin, tt.user.IsDefaultAdmin(); want != got {
			t.Errorf("Expected %t got %t", want, got)
		}
	}
}

func TestActiveDirectoryConfigSearchAttributes(t *testing.T) {
	config := ActiveDirectoryConfig{
		UserObjectClass:      "person",
		UserLoginAttribute:   "sAMAccountName",
		UserNameAttribute:    "name",
		UserEnabledAttribute: "userAccountControl",
		GroupObjectClass:     "group",
		GroupNameAttribute:   "name",
		GroupSearchAttribute: "sAMAccountName",
	}

	assert.Equal(t, []string{"person", "sAMAccountName", "name", "userAccountControl", "memberOf"}, config.GetUserSearchAttributes("memberOf"))
	assert.Equal(t, []string{"group", "sAMAccountName", "name", "sAMAccountName", "objectClass"}, config.GetGroupSearchAttributes("objectClass"))

	config.UserIDAttribute = "objectGUID"
	config.GroupIDAttribute = "objectSid"

	assert.Equal(t, []string{"person", "sAMAccountName", "name", "userAccountControl", "objectGUID", "memberOf"}, config.GetUserSearchAttributes("memberOf"))
	assert.Equal(t, []string{"group", "sAMAccountName", "name", "sAMAccountName", "objectSid", "objectClass"}, config.GetGroupSearchAttributes("objectClass"))
}

func TestLdapConfigSearchAttributes(t *testing.T) {
	config := LdapConfig{LdapFields: LdapFields{
		UserMemberAttribute:      "memberOf",
		GroupMemberUserAttribute: "entryDN",
		UserObjectClass:          "inetOrgPerson",
		UserLoginAttribute:       "uid",
		UserNameAttribute:        "cn",
		UserEnabledAttribute:     "nsAccountLock",
		GroupObjectClass:         "groupOfNames",
		GroupNameAttribute:       "cn",
		GroupSearchAttribute:     "cn",
	}}

	assert.Equal(t, []string{"dn", "memberOf", "inetOrgPerson", "uid", "cn", "nsAccountLock", "objectClass"}, config.GetUserSearchAttributes("objectClass"))
	assert.Equal(t, []string{"entryDN", "groupOfNames", "uid", "cn", "cn", "objectClass"}, config.GetGroupSearchAttributes("objectClass"))

	config.UserIDAttribute = "entryUUID"
	config.GroupIDAttribute = "gidNumber"

	assert.Equal(t, []string{"dn", "memberOf", "inetOrgPerson", "uid", "cn", "nsAccountLock", "entryUUID", "objectClass"}, config.GetUserSearchAttributes("objectClass"))
	assert.Equal(t, []string{"entryDN", "groupOfNames", "uid", "cn", "cn", "gidNumber", "objectClass"}, config.GetGroupSearchAttributes("objectClass"))
}

func TestTokenGetIsExpired(t *testing.T) {
	const day = int64(24 * time.Hour / time.Millisecond)

	tests := []struct {
		name      string
		createdAt time.Time
		ttl       int64
		want      bool
	}{
		{
			name:      "never expires",
			createdAt: time.Now().Add(-365 * 24 * time.Hour),
			ttl:       0,
			want:      false,
		},
		{
			name:      "not yet expired",
			createdAt: time.Now(),
			ttl:       day,
			want:      false,
		},
		{
			name:      "expired",
			createdAt: time.Now().Add(-2 * 24 * time.Hour),
			ttl:       day,
			want:      true,
		},
		{
			// Without saturation the duration wraps around to about -268 years.
			name:      "ttl above the range of time.Duration",
			createdAt: time.Now(),
			ttl:       10_000_000_000_000, // about 317 years
			want:      false,
		},
		{
			// Without saturation the duration wraps around to a fraction of a millisecond.
			name:      "ttl wrapping to a small positive duration",
			createdAt: time.Now().Add(-time.Hour),
			ttl:       18_446_744_073_710,
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := &Token{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(test.createdAt)},
				TTLMillis:  test.ttl,
			}
			assert.Equal(t, test.want, token.GetIsExpired())
		})
	}
}
