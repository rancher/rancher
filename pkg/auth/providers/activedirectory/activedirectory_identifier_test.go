package activedirectory

import (
	"errors"
	"fmt"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	ldapFakes "github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func identifierTestConfig() v3.ActiveDirectoryConfig {
	return v3.ActiveDirectoryConfig{
		ServiceAccountUsername:      saUsername,
		ServiceAccountPassword:      saPassword,
		UserObjectClass:             userObjectClassName,
		UserLoginAttribute:          "sAMAccountName",
		UserDisabledBitMask:         2,
		UserEnabledAttribute:        "userAccountControl",
		UserNameAttribute:           "name",
		UserSearchBase:              baseDN,
		GroupSearchBase:             "ou=groups,dc=foo,dc=bar",
		GroupDNAttribute:            "distinguishedName",
		GroupMemberMappingAttribute: "member",
		GroupMemberUserAttribute:    "distinguishedName",
		GroupNameAttribute:          "name",
		GroupObjectClass:            "group",
		GroupSearchAttribute:        "sAMAccountName",
		UserIDAttribute:             "sAMAccountName",
		GroupIDAttribute:            "sAMAccountName",
	}
}

func adUserEntry(dn, name, accountControl string, memberOf ...string) *ldapv3.Entry {
	return &ldapv3.Entry{
		DN: dn,
		Attributes: []*ldapv3.EntryAttribute{
			{Name: ObjectClass, Values: []string{"top", "person", "organizationalPerson", "user"}},
			{Name: "name", Values: []string{name}},
			{Name: "sAMAccountName", Values: []string{name}},
			{Name: "userAccountControl", Values: []string{accountControl}},
			{Name: MemberOfAttribute, Values: memberOf},
		},
	}
}

func adGroupEntry(dn, name string) *ldapv3.Entry {
	return &ldapv3.Entry{
		DN: dn,
		Attributes: []*ldapv3.EntryAttribute{
			{Name: ObjectClass, Values: []string{"top", "group"}},
			{Name: "name", Values: []string{name}},
			{Name: "sAMAccountName", Values: []string{name}},
			{Name: "objectSid", Values: []string{"S-1-5-" + name}},
		},
	}
}

func TestADProviderSearchPrincipalByAttribute(t *testing.T) {
	t.Parallel()

	provider := adProvider{}

	tests := []struct {
		name       string
		scope      string
		externalID string
		entries    []*ldapv3.Entry
		searchErr  error
		wantBase   string
		wantFilter string
		want       *v3.Principal
		wantErr    string
		wantNotFnd bool
	}{
		{
			name:       "user found",
			scope:      UserScope,
			externalID: "jdoe",
			entries:    []*ldapv3.Entry{adUserEntry("cn=John Doe,"+baseDN, "jdoe", "512")},
			wantBase:   baseDN,
			wantFilter: "(&(objectClass=person)(sAMAccountName=jdoe))",
			want: &v3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: "activedirectory_user://jdoe"},
				DisplayName:   "jdoe",
				LoginName:     "jdoe",
				PrincipalType: "user",
				Provider:      Name,
				Me:            true,
			},
		},
		{
			name:       "group found in group search base",
			scope:      GroupScope,
			externalID: "engineering",
			entries:    []*ldapv3.Entry{adGroupEntry("cn=Engineering,ou=groups,dc=foo,dc=bar", "engineering")},
			wantBase:   "ou=groups,dc=foo,dc=bar",
			wantFilter: "(&(objectClass=group)(sAMAccountName=engineering))",
			want: &v3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: "activedirectory_group://engineering"},
				DisplayName:   "engineering",
				LoginName:     "engineering",
				PrincipalType: "group",
				Provider:      Name,
				Me:            true,
			},
		},
		{
			name:       "filter value is escaped",
			scope:      UserScope,
			externalID: "j*doe(1)",
			wantBase:   baseDN,
			wantFilter: `(&(objectClass=person)(sAMAccountName=j\2adoe\281\29))`,
			wantNotFnd: true,
		},
		{
			name:       "not found is non-transient",
			scope:      UserScope,
			externalID: "ghost",
			wantBase:   baseDN,
			wantFilter: "(&(objectClass=person)(sAMAccountName=ghost))",
			wantNotFnd: true,
		},
		{
			name:       "multiple entries",
			scope:      UserScope,
			externalID: "jdoe",
			entries:    []*ldapv3.Entry{adUserEntry("cn=a,"+baseDN, "jdoe", "512"), adUserEntry("cn=b,"+baseDN, "jdoe", "512")},
			wantBase:   baseDN,
			wantFilter: "(&(objectClass=person)(sAMAccountName=jdoe))",
			wantErr:    "multiple entries found for sAMAccountName=jdoe",
		},
		{
			name:       "disabled user",
			scope:      UserScope,
			externalID: "jdoe",
			entries:    []*ldapv3.Entry{adUserEntry("cn=John Doe,"+baseDN, "jdoe", "514")},
			wantBase:   baseDN,
			wantFilter: "(&(objectClass=person)(sAMAccountName=jdoe))",
			wantErr:    "permission denied",
		},
		{
			name:       "search error",
			scope:      UserScope,
			externalID: "jdoe",
			searchErr:  ldapv3.NewError(ldapv3.LDAPResultUnavailable, fmt.Errorf("unavailable")),
			wantBase:   baseDN,
			wantFilter: "(&(objectClass=person)(sAMAccountName=jdoe))",
			wantErr:    "error searching for sAMAccountName=jdoe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := identifierTestConfig()
			var gotSearch *ldapv3.SearchRequest
			conn := &ldapFakes.FakeLdapConn{
				SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
					gotSearch = searchRequest
					if tt.searchErr != nil {
						return nil, tt.searchErr
					}
					return &ldapv3.SearchResult{Entries: tt.entries}, nil
				},
			}

			got, err := provider.searchPrincipalByAttribute(conn, tt.externalID, tt.scope, "sAMAccountName", &config)

			require.NotNil(t, gotSearch)
			assert.Equal(t, tt.wantBase, gotSearch.BaseDN)
			assert.Equal(t, ldapv3.ScopeWholeSubtree, gotSearch.Scope)
			assert.Equal(t, tt.wantFilter, gotSearch.Filter)
			assert.Contains(t, gotSearch.Attributes, "sAMAccountName")

			switch {
			case tt.wantNotFnd:
				var nonTransient *common.NonTransientError
				require.True(t, errors.As(err, &nonTransient), "want NonTransientError, got %v", err)
				assert.Nil(t, got)
			case tt.wantErr != "":
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestADProviderLoginUserWithIDAttributes(t *testing.T) {
	t.Parallel()

	const (
		groupDN  = "cn=group,ou=groups,dc=foo,dc=bar"
		parentDN = "cn=parent,ou=groups,dc=foo,dc=bar"
	)

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().CheckAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

	provider := adProvider{
		userMGR:  userManager,
		tokenMGR: &tokens.Manager{},
	}

	credentials := v3.BasicLogin{Username: userName, Password: userPassword}

	config := identifierTestConfig()
	nested := true
	config.NestedGroupMembershipEnabled = &nested
	// objectSid is not part of the attribute list Rancher requests by default.
	config.GroupIDAttribute = "objectSid"

	var pagingAttributes [][]string
	conn := &ldapFakes.FakeLdapConn{
		SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			switch searchRequest.Filter {
			case "(&(sAMAccountName=user))":
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{adUserEntry(userDN, "user", "512", groupDN)}}, nil
			case "(&(objectClass=group)(objectSid=S-1-5-group))":
				// Nested group traversal resolves the group identifier back to its DN.
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{{DN: groupDN}}}, nil
			}
			return &ldapv3.SearchResult{}, nil
		},
		SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
			pagingAttributes = append(pagingAttributes, searchRequest.Attributes)
			switch searchRequest.Filter {
			case "(&(objectClass=group)(|(distinguishedName=" + groupDN + ")))":
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{adGroupEntry(groupDN, "group")}}, nil
			case "(&(member=" + groupDN + ")(objectClass=group))":
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{adGroupEntry(parentDN, "parent")}}, nil
			}
			return &ldapv3.SearchResult{}, nil
		},
	}

	userPrincipal, groupPrincipals, err := provider.loginUser(conn, &credentials, &config)
	require.NoError(t, err)

	assert.Equal(t, "activedirectory_user://user", userPrincipal.Name)
	assert.Equal(t, "user", userPrincipal.LoginName)

	var groupNames []string
	for _, g := range groupPrincipals {
		groupNames = append(groupNames, g.Name)
	}
	assert.Equal(t, []string{"activedirectory_group://S-1-5-group", "activedirectory_group://S-1-5-parent"}, groupNames)

	require.Len(t, pagingAttributes, 3)
	for _, attrs := range pagingAttributes {
		assert.Contains(t, attrs, "objectSid")
	}
}

func TestADProviderGetGroupPrincipalsFromSearchBindFailure(t *testing.T) {
	t.Parallel()

	const groupDN = "cn=group,ou=groups,dc=foo,dc=bar"

	provider := adProvider{}
	conn := &ldapFakes.FakeLdapConn{
		BindFunc: func(username, password string) error {
			return ldapv3.NewError(ldapv3.LDAPResultInvalidCredentials, fmt.Errorf("invalid credentials"))
		},
	}

	t.Run("groups identified by DN fall back to memberOf", func(t *testing.T) {
		t.Parallel()

		config := identifierTestConfig()
		config.Enabled = true
		config.GroupIDAttribute = ""

		groups, err := provider.getGroupPrincipalsFromSearch(conn, &config, config.GroupSearchBase, "(objectClass=group)", []string{groupDN})
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, "activedirectory_group://"+groupDN, groups[0].Name)
	})

	t.Run("groups identified by attribute return the bind error", func(t *testing.T) {
		t.Parallel()

		config := identifierTestConfig()
		config.Enabled = true

		groups, err := provider.getGroupPrincipalsFromSearch(conn, &config, config.GroupSearchBase, "(objectClass=group)", []string{groupDN})
		require.Error(t, err)
		assert.True(t, ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultInvalidCredentials))
		assert.Empty(t, groups)
	})
}

func TestValidateIDAttributesUnchanged(t *testing.T) {
	t.Parallel()

	stored := &v3.ActiveDirectoryConfig{UserIDAttribute: "sAMAccountName", GroupIDAttribute: "sAMAccountName"}

	tests := []struct {
		name     string
		incoming v3.ActiveDirectoryConfig
		wantErr  string
	}{
		{name: "unchanged", incoming: *stored},
		{name: "user attribute changed", incoming: v3.ActiveDirectoryConfig{UserIDAttribute: "objectGUID", GroupIDAttribute: "sAMAccountName"}, wantErr: "userIDAttribute cannot be changed"},
		{name: "group attribute cleared", incoming: v3.ActiveDirectoryConfig{UserIDAttribute: "sAMAccountName"}, wantErr: "groupIDAttribute cannot be changed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateIDAttributesUnchanged(stored, &tt.incoming)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
