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
		attr string
		want string
	}{
		// Whitespace.
		{"", ""},
		{"   ", ""},
		{" a- b", "a-b"},
		{"a\tb", "ab"},
		{"a\nb", "ab"},
		// Special characters.
		{"a#b$c'd(e)f+g,h;i<j=k>l\\m_n{o}p", "abcdefghijklmnop"},
		// Valid short names stay the same.
		{"a", "a"},
		{"a1", "a1"},
		{"a1-", "a1-"},
		{"a-b", "a-b"},
		{"a1-b2", "a1-b2"},
		{"1a", "1a"},
		{"-a", "-a"},
		{"-1a", "-1a"},
		{"1-a", "1-a"},
		// Valid numeric OIDs stay the same.
		{"1", "1"},
		{"1.2", "1.2"},
		{"1.2.3", "1.2.3"},
		{"123.456.789", "123.456.789"},
		{"12345678901234567890", "12345678901234567890"},
		{"1.2.3.4.5.6.7.8.9.10.11.12.13.14.15.16.17.18.19.20", "1.2.3.4.5.6.7.8.9.10.11.12.13.14.15.16.17.18.19.20"},
		// Technically invalid identifiers.
		{"1ab", "1ab"},
		{"1.a.2", "1.a.2"},
		{".", "."},
		{"a.b", "a.b"},
		{"1-2-3", "1-2-3"},
	}

	for _, test := range tests {
		t.Run(test.attr, func(t *testing.T) {
			assert.Equal(t, test.want, SanitizeAttr(test.attr))
		})
	}
}
func TestIsValidAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attr  string
		valid bool
	}{
		{"", false},
		// Short names.
		{"a", true},
		{"a1", true},
		{"a1-", true},
		{"a-b", true},
		{"a1-b2", true},
		{"1a", false},
		{"-a", false},
		{"-1a", false},
		{"1-a", false},
		// Numeric OIDs.
		{"0", true},
		{"1", true},
		{"0.1", true},
		{"1.2", true},
		{"0.0.0", true},
		{"1.2.3", true},
		{"123.456.789", true},
		{"12345678901234567890", true},
		{"1.2.3.4.5.6.7.8.9.10.11.12.13.14.15.16.17.18.19.20", true},
		{".", false},
		{"1.", false},
		{"1..1", false},
		{"1.-1", false},
		{"01", false},
		{"1.02", false},
	}

	for _, test := range tests {
		t.Run(test.attr, func(t *testing.T) {
			assert.Equal(t, test.valid, IsValidAttr(test.attr))
		})
	}
}
