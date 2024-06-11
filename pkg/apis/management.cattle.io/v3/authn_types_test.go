package v3

import (
	"testing"
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
