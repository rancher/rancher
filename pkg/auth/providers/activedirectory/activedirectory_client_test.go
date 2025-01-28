package activedirectory

import (
	"fmt"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	ldapFakes "github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	saUsername          = "FOO\\sa"
	saPassword          = "secret"
	userName            = "user"
	baseDN              = "ou=foo,dc=foo,dc=bar"
	userDN              = "cn=user," + baseDN
	userPassword        = "secret"
	userObjectClassName = "person"
)

func TestADProviderLoginUser(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		ServiceAccountUsername:      saUsername,
		ServiceAccountPassword:      saPassword,
		UserObjectClass:             userObjectClassName,
		UserLoginAttribute:          "sAMAccountName",
		UserDisabledBitMask:         2,
		UserEnabledAttribute:        "userAccountControl",
		UserNameAttribute:           "name",
		UserSearchBase:              baseDN,
		GroupDNAttribute:            "distinguishedName",
		GroupMemberMappingAttribute: "member",
		GroupMemberUserAttribute:    "distinguishedName",
		GroupNameAttribute:          "name",
		GroupObjectClass:            "group",
		GroupSearchAttribute:        "sAMAccountName",
	}

	provider := adProvider{
		userMGR:  common.FakeUserManager{HasAccess: true},
		tokenMGR: &tokens.Manager{},
	}

	credentials := v3.BasicLogin{
		Username: userName,
		Password: userPassword,
	}

	userSearchResult := &ldapv3.SearchResult{
		Entries: []*ldapv3.Entry{
			{
				DN: userDN,
				Attributes: []*ldapv3.EntryAttribute{
					{Name: ObjectClass, Values: []string{"top", "person", "organizationalPerson", "user"}},
					{Name: "name", Values: []string{"user"}},
					{Name: "memberOf", Values: []string{"cn=group,ou=foo,dc=foo,dc=bar"}},
					{Name: "objectGUID", Values: []string{"\xff\xf9MyK0\xbaM\xb8vz#h^XP"}},
					{Name: "sAMAccountName", Values: []string{"user"}},
					{Name: "userAccountControl", Values: []string{"512"}},
				},
			},
		},
	}

	groupSearchResult := &ldapv3.SearchResult{
		Entries: []*ldapv3.Entry{
			{
				DN: "cn=group,ou=foo,dc=foo,dc=bar",
				Attributes: []*ldapv3.EntryAttribute{
					{Name: ObjectClass, Values: []string{"top", "group"}},
					{Name: "name", Values: []string{"group"}},
					{Name: "sAMAccountName", Values: []string{"group"}},
				},
			},
		},
	}

	t.Run("successful user login with login filter", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(sAMAccountName=user)(!(status=inactive)))" &&
					searchRequest.BaseDN == baseDN {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=group)(|(distinguishedName=cn=group,ou=foo,dc=foo,dc=bar)))" {
					return groupSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				boundCredentials = append(boundCredentials, v3.BasicLogin{Username: username, Password: password})
				return nil
			},
		}

		config := config
		config.UserLoginFilter = "(!(status=inactive))"

		wantUserPrincipal := v3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "activedirectory_user://cn=user,ou=foo,dc=foo,dc=bar",
			},
			DisplayName:   "user",
			LoginName:     "user",
			PrincipalType: "user",
			Provider:      "activedirectory",
			Me:            true,
		}
		wantGroupPrincipals := []v3.Principal{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "activedirectory_group://cn=group,ou=foo,dc=foo,dc=bar",
				},
				DisplayName:   "group",
				LoginName:     "group",
				PrincipalType: "group",
				Provider:      "activedirectory",
				Me:            true,
				MemberOf:      true,
			},
		}

		provider := provider

		userPrincipal, groupPrincipals, err := provider.loginUser(ldapConn, &credentials, &config)
		require.NoError(t, err)

		require.Len(t, boundCredentials, 3)
		assert.Equal(t, v3.BasicLogin{Username: saUsername, Password: saPassword}, boundCredentials[0])
		assert.Equal(t, v3.BasicLogin{Username: userName, Password: userPassword}, boundCredentials[1])
		assert.Equal(t, v3.BasicLogin{Username: saUsername, Password: saPassword}, boundCredentials[2])

		assert.Equal(t, wantUserPrincipal, userPrincipal)
		assert.Equal(t, wantGroupPrincipals, groupPrincipals)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(sAMAccountName=user)" &&
					searchRequest.BaseDN == baseDN {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				if username == userName && password != userPassword {
					return ldapv3.NewError(ldapv3.LDAPResultInvalidCredentials, fmt.Errorf("ldap: invalid credentials"))
				}
				boundCredentials = append(boundCredentials, v3.BasicLogin{Username: username, Password: password})
				return nil
			},
		}

		credentials := credentials
		credentials.Password = "invalid"

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)

		require.Len(t, boundCredentials, 1)
		assert.Equal(t, v3.BasicLogin{Username: saUsername, Password: saPassword}, boundCredentials[0])
	})

	t.Run("user has no access", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(sAMAccountName=user))" &&
					searchRequest.BaseDN == baseDN {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=group)(|(distinguishedName=cn=group,ou=foo,dc=foo,dc=bar)))" {
					return groupSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				boundCredentials = append(boundCredentials, v3.BasicLogin{Username: username, Password: password})
				return nil
			},
		}

		provider := provider
		provider.userMGR = common.FakeUserManager{HasAccess: false}

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.PermissionDenied, herr.Code)

		require.Len(t, boundCredentials, 3)
	})

	t.Run("missing password", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{}
		provider := provider

		credentials := credentials
		credentials.Password = ""

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.MissingRequired, herr.Code)
	})

	t.Run("invalid service account credentials", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			BindFunc: func(username, password string) error {
				return ldapv3.NewError(ldapv3.LDAPResultInvalidCredentials, fmt.Errorf("ldap: invalid credentials"))
			},
		}

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)
	})

	t.Run("error authenticating service account", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			BindFunc: func(username, password string) error {
				return ldapv3.NewError(ldapv3.LDAPResultServerDown, fmt.Errorf("ldap: server down"))
			},
		}

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.ServerError, herr.Code)
	})

	t.Run("no user found", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{}
		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)
	})

	t.Run("multiple users found", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				return &ldapv3.SearchResult{
					Entries: []*ldapv3.Entry{{}, {}}, // Return multiple entries.
				}, nil
			},
		}
		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)
	})

	t.Run("error authenticating user", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(sAMAccountName=user))" &&
					searchRequest.BaseDN == baseDN {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				if username == userName && password == userPassword {
					return ldapv3.NewError(ldapv3.LDAPResultServerDown, fmt.Errorf("ldap: server down"))
				}
				return nil
			},
		}

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.ServerError, herr.Code)
	})

	t.Run("error getting user details", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				return nil, ldapv3.NewError(ldapv3.LDAPResultUnavailable, fmt.Errorf("ldap: result unavailable"))
			},
		}

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)
	})

	t.Run("empty user details results", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				return &ldapv3.SearchResult{}, nil
			},
		}

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)
	})
}
