package ldap

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
)

const (
	usersBase  = "ou=users,dc=foo,dc=bar"
	groupsBase = "ou=groups,dc=foo,dc=bar"
)

func identifierTestConfig() v3.LdapConfig {
	return v3.LdapConfig{
		LdapFields: v3.LdapFields{
			ServiceAccountDistinguishedName: saDN,
			ServiceAccountPassword:          saPassword,
			UserObjectClass:                 userObjectClassName,
			UserLoginAttribute:              "uid",
			UserNameAttribute:               "cn",
			UserSearchBase:                  usersBase,
			GroupSearchBase:                 groupsBase,
			GroupDNAttribute:                "entryDN",
			GroupMemberMappingAttribute:     "member",
			GroupMemberUserAttribute:        "entryDN",
			GroupNameAttribute:              "cn",
			GroupObjectClass:                "groupOfNames",
			GroupSearchAttribute:            "cn",
			UserIDAttribute:                 "uid",
			GroupIDAttribute:                "cn",
		},
	}
}

func openldapProvider() ldapProvider {
	return ldapProvider{
		providerName: OpenLdapName,
		userScope:    OpenLdapName + "_user",
		groupScope:   OpenLdapName + "_group",
	}
}

func ldapUserEntry(dn, uid string) *ldapv3.Entry {
	return &ldapv3.Entry{
		DN: dn,
		Attributes: []*ldapv3.EntryAttribute{
			{Name: ObjectClass, Values: []string{userObjectClassName}},
			{Name: "cn", Values: []string{uid}},
			{Name: "uid", Values: []string{uid}},
			{Name: "entryDN", Values: []string{dn}},
		},
	}
}

func ldapGroupEntry(dn, cn string) *ldapv3.Entry {
	return &ldapv3.Entry{
		DN: dn,
		Attributes: []*ldapv3.EntryAttribute{
			{Name: ObjectClass, Values: []string{"groupOfNames"}},
			{Name: "cn", Values: []string{cn}},
			{Name: "entryDN", Values: []string{dn}},
			{Name: "gidNumber", Values: []string{map[string]string{"group": "1001", "parent": "1002"}[cn]}},
		},
	}
}

func TestLDAPProviderSearchPrincipalByAttribute(t *testing.T) {
	t.Parallel()

	provider := openldapProvider()

	tests := []struct {
		name       string
		scope      string
		attribute  string
		externalID string
		entries    []*ldapv3.Entry
		searchErr  error
		wantBase   string
		wantFilter string
		wantName   string
		wantErr    string
		wantNotFnd bool
	}{
		{
			name:       "user found",
			scope:      provider.userScope,
			attribute:  "uid",
			externalID: "jdoe",
			entries:    []*ldapv3.Entry{ldapUserEntry("cn=John Doe,"+usersBase, "jdoe")},
			wantBase:   usersBase,
			wantFilter: "(&(objectClass=inetOrgPerson)(uid=jdoe))",
			wantName:   "openldap_user://jdoe",
		},
		{
			name:       "group found in group search base",
			scope:      provider.groupScope,
			attribute:  "cn",
			externalID: "engineering",
			entries:    []*ldapv3.Entry{ldapGroupEntry("cn=engineering,"+groupsBase, "engineering")},
			wantBase:   groupsBase,
			wantFilter: "(&(objectClass=groupOfNames)(cn=engineering))",
			wantName:   "openldap_group://engineering",
		},
		{
			name:       "not found is non-transient",
			scope:      provider.userScope,
			attribute:  "uid",
			externalID: "ghost",
			wantBase:   usersBase,
			wantFilter: "(&(objectClass=inetOrgPerson)(uid=ghost))",
			wantNotFnd: true,
		},
		{
			name:       "multiple entries",
			scope:      provider.userScope,
			attribute:  "uid",
			externalID: "jdoe",
			entries:    []*ldapv3.Entry{ldapUserEntry("cn=a,"+usersBase, "jdoe"), ldapUserEntry("cn=b,"+usersBase, "jdoe")},
			wantBase:   usersBase,
			wantFilter: "(&(objectClass=inetOrgPerson)(uid=jdoe))",
			wantErr:    "multiple entries found for uid=jdoe",
		},
		{
			name:       "search error",
			scope:      provider.userScope,
			attribute:  "uid",
			externalID: "jdoe",
			searchErr:  ldapv3.NewError(ldapv3.LDAPResultUnavailable, fmt.Errorf("unavailable")),
			wantBase:   usersBase,
			wantFilter: "(&(objectClass=inetOrgPerson)(uid=jdoe))",
			wantErr:    "error searching for uid=jdoe",
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

			got, err := provider.searchPrincipalByAttribute(conn, tt.externalID, tt.scope, tt.attribute, &config)

			require.NotNil(t, gotSearch)
			assert.Equal(t, tt.wantBase, gotSearch.BaseDN)
			assert.Equal(t, ldapv3.ScopeWholeSubtree, gotSearch.Scope)
			assert.Equal(t, tt.wantFilter, gotSearch.Filter)
			assert.Contains(t, gotSearch.Attributes, tt.attribute)

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
				assert.Equal(t, tt.wantName, got.Name)
			}
		})
	}
}

func TestLDAPProviderSearchLdapIdentifier(t *testing.T) {
	t.Parallel()

	userEntry := ldapUserEntry("cn=John Doe,"+usersBase, "jdoe")
	groupEntry := ldapGroupEntry("cn=engineering,"+groupsBase, "engineering")

	tests := []struct {
		name             string
		providerName     string
		userIDAttribute  string
		groupIDAttribute string
		wantUser         string
		wantGroup        string
	}{
		{
			name:         "openldap defaults to DN",
			providerName: OpenLdapName,
			wantUser:     "openldap_user://cn=John Doe," + usersBase,
			wantGroup:    "openldap_group://cn=engineering," + groupsBase,
		},
		{
			name:             "openldap with identifier attributes",
			providerName:     OpenLdapName,
			userIDAttribute:  "uid",
			groupIDAttribute: "cn",
			wantUser:         "openldap_user://jdoe",
			wantGroup:        "openldap_group://engineering",
		},
		{
			name:         "shibboleth defaults to login attribute and group DN attribute",
			providerName: ShibbolethName,
			wantUser:     "shibboleth_user://jdoe",
			wantGroup:    "shibboleth_group://cn=engineering," + groupsBase,
		},
		{
			name:             "shibboleth identifier attributes take precedence",
			providerName:     ShibbolethName,
			userIDAttribute:  "entryDN",
			groupIDAttribute: "cn",
			wantUser:         "shibboleth_user://cn=John Doe," + usersBase,
			wantGroup:        "shibboleth_group://engineering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := ldapProvider{
				providerName: tt.providerName,
				userScope:    tt.providerName + "_user",
				groupScope:   tt.providerName + "_group",
			}
			config := identifierTestConfig()
			config.UserIDAttribute = tt.userIDAttribute
			config.GroupIDAttribute = tt.groupIDAttribute

			conn := &ldapFakes.FakeLdapConn{
				SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
					switch searchRequest.BaseDN {
					case usersBase:
						return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{userEntry}}, nil
					case groupsBase:
						return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{groupEntry}}, nil
					}
					return &ldapv3.SearchResult{}, nil
				},
			}

			users, err := provider.searchLdap("(objectClass=inetOrgPerson)", provider.userScope, &config, conn)
			require.NoError(t, err)
			require.Len(t, users, 1)
			assert.Equal(t, tt.wantUser, users[0].Name)

			groups, err := provider.searchLdap("(objectClass=groupOfNames)", provider.groupScope, &config, conn)
			require.NoError(t, err)
			require.Len(t, groups, 1)
			assert.Equal(t, tt.wantGroup, groups[0].Name)
		})
	}
}

func TestLDAPProviderLoginUserWithIDAttributes(t *testing.T) {
	t.Parallel()

	const (
		groupDN  = "cn=group," + groupsBase
		parentDN = "cn=parent," + groupsBase
	)

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().CheckAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

	provider := openldapProvider()
	provider.userMGR = userManager
	provider.tokenMGR = &tokens.Manager{}

	config := identifierTestConfig()
	config.NestedGroupMembershipEnabled = true
	// gidNumber is not part of the attribute list Rancher requests by default.
	config.GroupIDAttribute = "gidNumber"

	var pagingAttributes [][]string
	conn := &ldapFakes.FakeLdapConn{
		SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			switch {
			case searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" && searchRequest.BaseDN == usersBase,
				searchRequest.Filter == "(objectClass=inetOrgPerson)" && searchRequest.BaseDN == userDN:
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{ldapUserEntry(userDN, "user")}}, nil
			case searchRequest.Filter == "(&(objectClass=groupOfNames)(gidNumber=1001))":
				// Nested group traversal resolves the group identifier back to its DN.
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{{DN: groupDN}}}, nil
			}
			return &ldapv3.SearchResult{}, nil
		},
		SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
			pagingAttributes = append(pagingAttributes, searchRequest.Attributes)
			switch searchRequest.Filter {
			case "(&(member=" + userDN + ")(objectClass=groupOfNames))":
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{ldapGroupEntry(groupDN, "group")}}, nil
			case "(&(member=" + groupDN + ")(objectClass=groupOfNames))":
				return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{ldapGroupEntry(parentDN, "parent")}}, nil
			}
			return &ldapv3.SearchResult{}, nil
		},
	}

	credentials := v3.BasicLogin{Username: userName, Password: userPassword}
	userPrincipal, groupPrincipals, err := provider.loginUser(conn, &credentials, &config)
	require.NoError(t, err)

	assert.Equal(t, "openldap_user://user", userPrincipal.Name)

	var groupNames []string
	for _, g := range groupPrincipals {
		groupNames = append(groupNames, g.Name)
	}
	assert.Equal(t, []string{"openldap_group://1001", "openldap_group://1002"}, groupNames)

	require.Len(t, pagingAttributes, 3)
	for _, attrs := range pagingAttributes {
		assert.Contains(t, attrs, "gidNumber")
	}
}

func TestValidateIDAttributesUnchanged(t *testing.T) {
	t.Parallel()

	stored := &v3.LdapConfig{LdapFields: v3.LdapFields{UserIDAttribute: "uid", GroupIDAttribute: "cn"}}

	tests := []struct {
		name     string
		incoming v3.LdapFields
		wantErr  string
	}{
		{name: "unchanged", incoming: stored.LdapFields},
		{name: "user attribute changed", incoming: v3.LdapFields{UserIDAttribute: "entryUUID", GroupIDAttribute: "cn"}, wantErr: "userIDAttribute cannot be changed"},
		{name: "group attribute cleared", incoming: v3.LdapFields{UserIDAttribute: "uid"}, wantErr: "groupIDAttribute cannot be changed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateIDAttributesUnchanged(stored, &v3.LdapConfig{LdapFields: tt.incoming})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLDAPProviderSamlSearchExternalID(t *testing.T) {
	t.Parallel()

	provider := ldapProvider{
		providerName: ShibbolethName,
		userScope:    ShibbolethName + "_user",
		groupScope:   ShibbolethName + "_group",
	}
	config := identifierTestConfig()

	userEntry := ldapUserEntry("cn=John Doe,"+usersBase, "jdoe")
	groupEntry := ldapGroupEntry("cn=engineering,"+groupsBase, "engineering")
	bareEntry := &ldapv3.Entry{DN: "cn=bare," + usersBase}

	assert.Equal(t, "jdoe", provider.samlSearchExternalID(userEntry, provider.userScope, &config))
	assert.Equal(t, "cn=engineering,"+groupsBase, provider.samlSearchExternalID(groupEntry, provider.groupScope, &config))
	// Entries without the attribute keep using their DN, as they always have.
	assert.Equal(t, "cn=bare,"+usersBase, provider.samlSearchExternalID(bareEntry, provider.userScope, &config))
	assert.Equal(t, "cn=bare,"+usersBase, provider.samlSearchExternalID(bareEntry, provider.groupScope, &config))
}
