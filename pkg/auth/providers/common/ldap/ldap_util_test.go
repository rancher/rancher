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

func TestSanitizeAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"a", "a"},
		{"a1", "a1"},
		{"a-b", "a-b"},
		{" a- b", "a-b"},
		{"-ab", "ab"},
		{"a!()&|", "a"},
		{"a_b", "ab"},
		{"1ab", "ab"},
		{" 1ab", "ab"},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, SanitizeAttr(test.in))
		})
	}
}
func TestIsValidAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in    string
		valid bool
	}{
		{"a", true},
		{"a1", true},
		{"a1-", true},
		{"a-b", true},
		{"a1-b2", true},
		{"", false},
		{"1a", false},
		{"-a", false},
		{"-1a", false},
		{"1-a", false},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.valid, IsValidAttr(test.in))
		})
	}
}
