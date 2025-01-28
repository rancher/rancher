package ldap

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
	saDN                = "cn=sa,ou=users,dc=foo,dc=bar"
	saPassword          = "secret"
	userName            = "user"
	userDN              = "cn=user,ou=users,dc=foo,dc=bar"
	userPassword        = "secret"
	userObjectClassName = "inetOrgPerson"
)

func TestLDAPProviderLoginUser(t *testing.T) {
	t.Parallel()

	config := v3.LdapConfig{
		LdapFields: v3.LdapFields{
			ServiceAccountDistinguishedName: saDN,
			ServiceAccountPassword:          saPassword,
			UserObjectClass:                 userObjectClassName,
			UserLoginAttribute:              "uid",
			UserNameAttribute:               "cn",
			UserSearchBase:                  "ou=users,dc=foo,dc=bar",
			GroupDNAttribute:                "entryDN",
			GroupMemberMappingAttribute:     "member",
			GroupMemberUserAttribute:        "entryDN",
			GroupNameAttribute:              "cn",
			GroupObjectClass:                "groupOfNames",
			GroupSearchAttribute:            "cn",
		},
	}

	provider := ldapProvider{
		providerName: "openldap",
		userMGR:      common.FakeUserManager{HasAccess: true},
		tokenMGR:     &tokens.Manager{},
		userScope:    "openldap_user",
		groupScope:   "openldap_group",
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
					{Name: ObjectClass, Values: []string{userObjectClassName}},
					{Name: "cn", Values: []string{"user"}},
					{Name: "uid", Values: []string{"user"}},
				},
			},
		},
	}
	userDetailsResult := &ldapv3.SearchResult{
		Entries: []*ldapv3.Entry{
			{
				DN: userDN,
				Attributes: []*ldapv3.EntryAttribute{
					{Name: ObjectClass, Values: []string{userObjectClassName}},
					{Name: "cn", Values: []string{"user"}},
					{Name: "uid", Values: []string{"user"}},
					{Name: "entryDN", Values: []string{userDN}},
				},
			},
		},
	}
	groupSearchResult := &ldapv3.SearchResult{
		Entries: []*ldapv3.Entry{
			{
				DN: "cn=group,ou=groups,dc=foo,dc=bar",
				Attributes: []*ldapv3.EntryAttribute{
					{Name: ObjectClass, Values: []string{"groupOfNames"}},
					{Name: "cn", Values: []string{"group"}},
					{Name: "entryDN", Values: []string{"cn=group,ou=groups,dc=foo,dc=bar"}},
				},
			},
		},
	}

	t.Run("successful user login with login filter", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user)(!(status=inactive)))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				if searchRequest.Filter == "(objectClass=inetOrgPerson)" &&
					searchRequest.BaseDN == userDN {
					return userDetailsResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(member=cn=user,ou=users,dc=foo,dc=bar)(objectClass=groupOfNames))" {
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
				Name: "openldap_user://cn=user,ou=users,dc=foo,dc=bar",
			},
			DisplayName:   "user",
			LoginName:     "user",
			PrincipalType: "user",
			Provider:      "openldap",
			Me:            true,
		}
		wantGroupPrincipals := []v3.Principal{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openldap_group://cn=group,ou=groups,dc=foo,dc=bar",
				},
				DisplayName:   "group",
				LoginName:     "group",
				PrincipalType: "group",
				Provider:      "openldap",
				Me:            true,
			},
		}

		provider := provider

		userPrincipal, groupPrincipals, err := provider.loginUser(ldapConn, &credentials, &config)
		require.NoError(t, err)

		require.Len(t, boundCredentials, 3)
		// Bind as SA to See if the user exists and is unique.
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
		// Bind as a user to authenticate the user.
		assert.Equal(t, v3.BasicLogin{Username: userDN, Password: userPassword}, boundCredentials[1])
		// Bind back as SA to get principals from the search results via (ldapSearch).
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])

		assert.Equal(t, wantUserPrincipal, userPrincipal)
		assert.Equal(t, wantGroupPrincipals, groupPrincipals)
	})

	t.Run("invalid user credentials", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				if username == userDN && password != userPassword {
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
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
	})

	t.Run("user has no access", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				if searchRequest.Filter == "(objectClass=inetOrgPerson)" &&
					searchRequest.BaseDN == userDN {
					return userDetailsResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(member=cn=user,ou=users,dc=foo,dc=bar)(objectClass=groupOfNames))" {
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

	t.Run("successful user login with SearchUsingServiceAccount true", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				if searchRequest.Filter == "(objectClass=inetOrgPerson)" &&
					searchRequest.BaseDN == userDN {
					return userDetailsResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(member=cn=user,ou=users,dc=foo,dc=bar)(objectClass=groupOfNames))" {
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
		config.SearchUsingServiceAccount = true

		wantUserPrincipal := v3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "openldap_user://cn=user,ou=users,dc=foo,dc=bar",
			},
			DisplayName:   "user",
			LoginName:     "user",
			PrincipalType: "user",
			Provider:      "openldap",
			Me:            true,
		}
		wantGroupPrincipals := []v3.Principal{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openldap_group://cn=group,ou=groups,dc=foo,dc=bar",
				},
				DisplayName:   "group",
				LoginName:     "group",
				PrincipalType: "group",
				Provider:      "openldap",
				Me:            true,
			},
		}

		provider := provider

		userPrincipal, groupPrincipals, err := provider.loginUser(ldapConn, &credentials, &config)
		require.NoError(t, err)

		require.Len(t, boundCredentials, 4)
		// See if user exists and is unique.
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
		// Authenticate as a user.
		assert.Equal(t, v3.BasicLogin{Username: userDN, Password: userPassword}, boundCredentials[1])
		// Rebind as a service account before retrieve the user record by searching using user's DN.
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
		// Get principals from the search results via p.ldapSearch.
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])

		assert.Equal(t, wantUserPrincipal, userPrincipal)
		assert.Equal(t, wantGroupPrincipals, groupPrincipals)
	})

	t.Run("user login with invalid credentials with SearchUsingServiceAccount true", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v3.BasicLogin

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				if searchRequest.Filter == "(objectClass=inetOrgPerson)" &&
					searchRequest.BaseDN == userDN {
					return userDetailsResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			SearchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(member=cn=user,ou=users,dc=foo,dc=bar)(objectClass=groupOfNames))" {
					return groupSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				if username == userDN && password != userPassword {
					return ldapv3.NewError(ldapv3.LDAPResultInvalidCredentials, fmt.Errorf("ldap: invalid credentials"))
				}
				boundCredentials = append(boundCredentials, v3.BasicLogin{Username: username, Password: password})
				return nil
			},
		}

		config := config
		config.SearchUsingServiceAccount = true

		credentials := credentials
		credentials.Password = "invalid"

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)

		require.Len(t, boundCredentials, 1)
		assert.Equal(t, v3.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
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
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			BindFunc: func(username, password string) error {
				if username == userDN && password == userPassword {
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
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				if searchRequest.Filter == "(objectClass=inetOrgPerson)" &&
					searchRequest.BaseDN == userDN {
					return nil, ldapv3.NewError(ldapv3.LDAPResultUnavailable, fmt.Errorf("ldap: result unavailable"))
				}

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

	t.Run("empty user details results", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{
			SearchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}
				// Return empty result for the second (user details) request.
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
