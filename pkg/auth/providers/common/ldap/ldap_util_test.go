package ldap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserExternalID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		username    string
		loginDomain string
		want        string
	}{
		{
			desc:        "added login domain",
			username:    "user1",
			loginDomain: "Domain1",
			want:        "Domain1\\user1",
		},
		{
			desc:        "no login domain",
			username:    "user1",
			loginDomain: "",
			want:        "user1",
		},
		{
			desc:        "username already contains domain",
			username:    "Domain2\\user1",
			loginDomain: "Domain1",
			want:        "Domain2\\user1",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			externalID := GetUserExternalID(test.username, test.loginDomain)
			assert.Equal(t, test.want, externalID)
		})
	}
}
