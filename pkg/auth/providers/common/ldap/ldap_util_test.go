package ldap

import (
	"fmt"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func TestAttributesToPrincipal(t *testing.T) {
	t.Parallel()

	userAttribs := []*ldapv3.EntryAttribute{
		ldapv3.NewEntryAttribute("objectClass", []string{"person"}),
		ldapv3.NewEntryAttribute("name", []string{"John Doe"}),
		ldapv3.NewEntryAttribute("sAMAccountName", []string{"jdoe"}),
		ldapv3.NewEntryAttribute("employeeID", []string{"EMP-42"}),
	}

	groupAttribs := []*ldapv3.EntryAttribute{
		ldapv3.NewEntryAttribute("objectClass", []string{"group"}),
		ldapv3.NewEntryAttribute("name", []string{"Engineering"}),
		ldapv3.NewEntryAttribute("sAMAccountName", []string{"engineering"}),
	}

	tests := []struct {
		name                string
		attribs             []*ldapv3.EntryAttribute
		dn                  string
		scope               string
		identifierAttribute string
		wantName            string
		wantDisplayName     string
		wantErr             bool
	}{
		{
			name:                "user with default DN identifier",
			attribs:             userAttribs,
			dn:                  "cn=John Doe,ou=Users,dc=example,dc=com",
			scope:               "activedirectory_user",
			identifierAttribute: "",
			wantName:            "activedirectory_user://cn=John Doe,ou=Users,dc=example,dc=com",
			wantDisplayName:     "John Doe",
		},
		{
			name:                "user with sAMAccountName identifier",
			attribs:             userAttribs,
			dn:                  "cn=John Doe,ou=Users,dc=example,dc=com",
			scope:               "activedirectory_user",
			identifierAttribute: "sAMAccountName",
			wantName:            "activedirectory_user://jdoe",
			wantDisplayName:     "John Doe",
		},
		{
			name:                "user with employeeID identifier",
			attribs:             userAttribs,
			dn:                  "cn=John Doe,ou=Users,dc=example,dc=com",
			scope:               "activedirectory_user",
			identifierAttribute: "employeeID",
			wantName:            "activedirectory_user://EMP-42",
			wantDisplayName:     "John Doe",
		},
		{
			name:                "user with missing identifier attribute returns error",
			attribs:             userAttribs,
			dn:                  "cn=John Doe,ou=Users,dc=example,dc=com",
			scope:               "activedirectory_user",
			identifierAttribute: "nonExistentAttr",
			wantErr:             true,
		},
		{
			name:                "group with default DN identifier",
			attribs:             groupAttribs,
			dn:                  "cn=Engineering,ou=Groups,dc=example,dc=com",
			scope:               "activedirectory_group",
			identifierAttribute: "",
			wantName:            "activedirectory_group://cn=Engineering,ou=Groups,dc=example,dc=com",
			wantDisplayName:     "Engineering",
		},
		{
			name:                "group with sAMAccountName identifier",
			attribs:             groupAttribs,
			dn:                  "cn=Engineering,ou=Groups,dc=example,dc=com",
			scope:               "activedirectory_group",
			identifierAttribute: "sAMAccountName",
			wantName:            "activedirectory_group://engineering",
			wantDisplayName:     "Engineering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			principal, err := AttributesToPrincipal(
				tt.attribs, tt.dn, tt.scope, "activedirectory",
				"person", "name", "sAMAccountName", "group", "name",
				tt.identifierAttribute,
			)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, principal.Name)
			assert.Equal(t, tt.wantDisplayName, principal.DisplayName)
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

func TestNewIdentifierSearchRequest(t *testing.T) {
	t.Parallel()

	search := NewIdentifierSearchRequest("ou=groups,dc=example,dc=com", "group", "sAMAccountName", "eng(x)*", []string{"cn", "sAMAccountName"})

	assert.Equal(t, "ou=groups,dc=example,dc=com", search.BaseDN)
	assert.Equal(t, ldapv3.ScopeWholeSubtree, search.Scope)
	assert.Equal(t, `(&(objectClass=group)(sAMAccountName=eng\28x\29\2a))`, search.Filter)
	assert.Equal(t, []string{"cn", "sAMAccountName"}, search.Attributes)
}

func TestResolveIdentifierToDN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []*ldapv3.Entry
		err     error
		wantDN  string
		wantErr string
	}{
		{
			name:    "single entry",
			entries: []*ldapv3.Entry{{DN: "cn=Engineering,ou=Groups,dc=example,dc=com"}},
			wantDN:  "cn=Engineering,ou=Groups,dc=example,dc=com",
		},
		{
			name:    "no entries",
			wantErr: "no entry found for sAMAccountName=engineering",
		},
		{
			name:    "multiple entries",
			entries: []*ldapv3.Entry{{DN: "cn=a,dc=example,dc=com"}, {DN: "cn=b,dc=example,dc=com"}},
			wantErr: "multiple entries found for sAMAccountName=engineering",
		},
		{
			name:    "search error",
			err:     ldapv3.NewError(ldapv3.LDAPResultUnavailable, fmt.Errorf("unavailable")),
			wantErr: "error resolving identifier sAMAccountName=engineering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotSearch *ldapv3.SearchRequest
			conn := &FakeLdapConn{
				SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
					gotSearch = searchRequest
					return &ldapv3.SearchResult{Entries: tt.entries}, tt.err
				},
			}

			dn, err := ResolveIdentifierToDN("dc=example,dc=com", "group", "sAMAccountName", "engineering", conn)

			require.NotNil(t, gotSearch)
			assert.Equal(t, "dc=example,dc=com", gotSearch.BaseDN)
			assert.Equal(t, "(&(objectClass=group)(sAMAccountName=engineering))", gotSearch.Filter)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDN, dn)
		})
	}
}

func TestGatherParentGroups(t *testing.T) {
	t.Parallel()

	const (
		searchDomain = "dc=example,dc=com"
		groupScope   = "activedirectory_group"
		groupDN      = "cn=group,ou=Groups,dc=example,dc=com"
		parentDN     = "cn=parent,ou=Groups,dc=example,dc=com"
		grandDN      = "cn=grand,ou=Groups,dc=example,dc=com"
	)

	groupEntry := func(dn, name string) *ldapv3.Entry {
		return &ldapv3.Entry{
			DN: dn,
			Attributes: []*ldapv3.EntryAttribute{
				{Name: "objectClass", Values: []string{"top", "group"}},
				{Name: "name", Values: []string{name}},
				{Name: "sAMAccountName", Values: []string{name}},
			},
		}
	}

	// group is a member of parent, parent is a member of grand, grand is a member of group (cycle).
	parentsOf := map[string][]*ldapv3.Entry{
		groupDN:  {groupEntry(parentDN, "parent")},
		parentDN: {groupEntry(grandDN, "grand")},
		grandDN:  {groupEntry(groupDN, "group")},
	}

	baseConfig := ConfigAttributes{
		GroupMemberMappingAttribute: "member",
		GroupNameAttribute:          "name",
		GroupObjectClass:            "group",
		GroupSearchAttribute:        "sAMAccountName",
		ObjectClass:                 "objectClass",
		ProviderName:                "activedirectory",
		UserLoginAttribute:          "sAMAccountName",
		UserNameAttribute:           "name",
		UserObjectClass:             "person",
	}
	searchAttributes := []string{"memberOf", "objectClass", "group", "sAMAccountName", "name"}

	principalNames := func(principals []v3.Principal) []string {
		var names []string
		for _, p := range principals {
			names = append(names, p.Name)
		}
		return names
	}

	t.Run("groups identified by DN", func(t *testing.T) {
		t.Parallel()

		var pagingFilters []string
		conn := &FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				t.Fatalf("unexpected non-paging search %q", searchRequest.Filter)
				return nil, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				pagingFilters = append(pagingFilters, searchRequest.Filter)
				assert.Equal(t, searchAttributes, searchRequest.Attributes)
				for dn, parents := range parentsOf {
					if searchRequest.Filter == fmt.Sprintf("(&(member=%s)(objectClass=group))", ldapv3.EscapeFilter(dn)) {
						return &ldapv3.SearchResult{Entries: parents}, nil
					}
				}
				return &ldapv3.SearchResult{}, nil
			},
		}

		config := baseConfig
		groupMap := map[string]bool{}
		var nested []v3.Principal

		err := GatherParentGroups(v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: groupScope + "://" + groupDN}}, searchDomain, groupScope, &config, conn, groupMap, &nested, searchAttributes)
		require.NoError(t, err)

		assert.Equal(t, []string{groupScope + "://" + parentDN, groupScope + "://" + grandDN}, principalNames(nested))
		assert.Len(t, pagingFilters, 3)
	})

	t.Run("groups identified by attribute", func(t *testing.T) {
		t.Parallel()

		var resolveFilters, pagingFilters []string
		conn := &FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				resolveFilters = append(resolveFilters, searchRequest.Filter)
				assert.Equal(t, searchDomain, searchRequest.BaseDN)
				if searchRequest.Filter == "(&(objectClass=group)(sAMAccountName=group))" {
					return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{{DN: groupDN}}}, nil
				}
				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				pagingFilters = append(pagingFilters, searchRequest.Filter)
				assert.Equal(t, append(append([]string{}, searchAttributes...), "sAMAccountName"), searchRequest.Attributes)
				for dn, parents := range parentsOf {
					if searchRequest.Filter == fmt.Sprintf("(&(member=%s)(objectClass=group))", ldapv3.EscapeFilter(dn)) {
						return &ldapv3.SearchResult{Entries: parents}, nil
					}
				}
				return &ldapv3.SearchResult{}, nil
			},
		}

		config := baseConfig
		config.GroupIDAttribute = "sAMAccountName"
		groupMap := map[string]bool{}
		var nested []v3.Principal

		err := GatherParentGroups(v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: groupScope + "://group"}}, searchDomain, groupScope, &config, conn, groupMap, &nested, searchAttributes)
		require.NoError(t, err)

		assert.Equal(t, []string{groupScope + "://parent", groupScope + "://grand"}, principalNames(nested))
		// Only the starting group needs its identifier resolved to a DN; parents come back with their DN.
		assert.Equal(t, []string{"(&(objectClass=group)(sAMAccountName=group))"}, resolveFilters)
		assert.Len(t, pagingFilters, 3)
		// The caller's attribute list is left untouched.
		assert.Equal(t, []string{"memberOf", "objectClass", "group", "sAMAccountName", "name"}, searchAttributes)
	})

	t.Run("group identifier cannot be resolved", func(t *testing.T) {
		t.Parallel()

		conn := &FakeLdapConn{
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				t.Fatalf("unexpected paging search %q", searchRequest.Filter)
				return nil, nil
			},
		}

		config := baseConfig
		config.GroupIDAttribute = "sAMAccountName"
		var nested []v3.Principal

		err := GatherParentGroups(v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: groupScope + "://missing"}}, searchDomain, groupScope, &config, conn, map[string]bool{}, &nested, searchAttributes)
		require.ErrorContains(t, err, `failed to resolve group identifier "missing" to DN`)
		assert.Empty(t, nested)
	})
}
