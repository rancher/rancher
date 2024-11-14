package ldap

import (
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
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
		LdapFields: v32.LdapFields{
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
		userMGR:      mockUserManager{hasAccess: true},
		tokenMGR:     &tokens.Manager{},
		userScope:    "openldap_user",
		groupScope:   "openldap_group",
	}

	credentials := v32.BasicLogin{
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

		var boundCredentials []v32.BasicLogin

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
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
			searchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(member=cn=user,ou=users,dc=foo,dc=bar)(objectClass=groupOfNames))" {
					return groupSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			bindFunc: func(username, password string) error {
				boundCredentials = append(boundCredentials, v32.BasicLogin{Username: username, Password: password})
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
		assert.Equal(t, v32.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
		assert.Equal(t, v32.BasicLogin{Username: userDN, Password: userPassword}, boundCredentials[1])
		assert.Equal(t, v32.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])

		assert.Equal(t, wantUserPrincipal, userPrincipal)
		assert.Equal(t, wantGroupPrincipals, groupPrincipals)
	})

	t.Run("invalid user credentials", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v32.BasicLogin

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			searchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				return &ldapv3.SearchResult{}, nil
			},
			bindFunc: func(username, password string) error {
				boundCredentials = append(boundCredentials, v32.BasicLogin{Username: username, Password: password})
				if username == userDN && password == userPassword {
					return ldapv3.NewError(ldapv3.LDAPResultInvalidCredentials, fmt.Errorf("ldap: invalid credentials"))
				}
				return nil
			},
		}

		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)

		require.Len(t, boundCredentials, 2)
		assert.Equal(t, v32.BasicLogin{Username: saDN, Password: saPassword}, boundCredentials[0])
	})

	t.Run("user has no access", func(t *testing.T) {
		t.Parallel()

		var boundCredentials []v32.BasicLogin

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
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
			searchWithPagingFunc: func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(member=cn=user,ou=users,dc=foo,dc=bar)(objectClass=groupOfNames))" {
					return groupSearchResult, nil
				}
				return &ldapv3.SearchResult{}, nil
			},
			bindFunc: func(username, password string) error {
				boundCredentials = append(boundCredentials, v32.BasicLogin{Username: username, Password: password})
				return nil
			},
		}

		provider := provider
		provider.userMGR = mockUserManager{hasAccess: false}

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.PermissionDenied, herr.Code)

		require.Len(t, boundCredentials, 3)
	})

	t.Run("missing password", func(t *testing.T) {
		t.Parallel()

		ldapConn := &mockLdapConn{}
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

		ldapConn := &mockLdapConn{
			bindFunc: func(username, password string) error {
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

		ldapConn := &mockLdapConn{
			bindFunc: func(username, password string) error {
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

		ldapConn := &mockLdapConn{}
		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*httperror.APIError)
		require.True(t, ok)
		require.Equal(t, httperror.Unauthorized, herr.Code)
	})

	t.Run("multiple users found", func(t *testing.T) {
		t.Parallel()

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
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

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
				if searchRequest.Filter == "(&(objectClass=inetOrgPerson)(uid=user))" &&
					searchRequest.BaseDN == "ou=users,dc=foo,dc=bar" {
					return userSearchResult, nil
				}

				return &ldapv3.SearchResult{}, nil
			},
			bindFunc: func(username, password string) error {
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

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
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

		ldapConn := &mockLdapConn{
			searchFunc: func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
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

type mockLdapConn struct {
	bindFunc             func(username, password string) error
	searchFunc           func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error)
	searchWithPagingFunc func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error)
}

func (m *mockLdapConn) Start() {
	panic("unimplemented")
}
func (m *mockLdapConn) StartTLS(*tls.Config) error {
	panic("unimplemented")
}
func (m *mockLdapConn) Close() {
	panic("unimplemented")
}
func (m *mockLdapConn) IsClosing() bool {
	panic("unimplemented")
}
func (m *mockLdapConn) SetTimeout(time.Duration) {
	panic("unimplemented")
}
func (m *mockLdapConn) Bind(username, password string) error {
	if m.bindFunc != nil {
		return m.bindFunc(username, password)
	}
	return nil
}
func (m *mockLdapConn) UnauthenticatedBind(username string) error {
	panic("unimplemented")
}
func (m *mockLdapConn) SimpleBind(*ldapv3.SimpleBindRequest) (*ldapv3.SimpleBindResult, error) {
	panic("unimplemented")
}
func (m *mockLdapConn) ExternalBind() error {
	panic("unimplemented")
}
func (m *mockLdapConn) Add(*ldapv3.AddRequest) error {
	panic("unimplemented")
}
func (m *mockLdapConn) Del(*ldapv3.DelRequest) error {
	panic("unimplemented")
}
func (m *mockLdapConn) Modify(*ldapv3.ModifyRequest) error {
	panic("unimplemented")
}
func (m *mockLdapConn) ModifyDN(*ldapv3.ModifyDNRequest) error {
	panic("unimplemented")
}
func (m *mockLdapConn) ModifyWithResult(*ldapv3.ModifyRequest) (*ldapv3.ModifyResult, error) {
	panic("unimplemented")
}
func (m *mockLdapConn) Compare(dn, attribute, value string) (bool, error) {
	panic("unimplemented")
}
func (m *mockLdapConn) PasswordModify(*ldapv3.PasswordModifyRequest) (*ldapv3.PasswordModifyResult, error) {
	panic("unimplemented")
}
func (m *mockLdapConn) Search(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
	if m.searchFunc != nil {
		return m.searchFunc(searchRequest)
	}
	return &ldapv3.SearchResult{}, nil
}
func (m *mockLdapConn) SearchWithPaging(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
	if m.searchWithPagingFunc != nil {
		return m.searchWithPagingFunc(searchRequest, pagingSize)
	}
	return &ldapv3.SearchResult{}, nil
}

type mockUserManager struct {
	hasAccess bool
}

func (m mockUserManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) GetUser(apiContext *types.APIContext) string {
	panic("unimplemented")
}
func (m mockUserManager) EnsureToken(input user.TokenInput) (string, error) {
	panic("unimplemented")
}
func (m mockUserManager) EnsureClusterToken(clusterName string, input user.TokenInput) (string, error) {
	panic("unimplemented")
}
func (m mockUserManager) DeleteToken(tokenName string) error {
	panic("unimplemented")
}
func (m mockUserManager) EnsureUser(principalName, displayName string) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error) {
	return m.hasAccess, nil
}
func (m mockUserManager) SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error {
	panic("unimplemented")
}
func (m mockUserManager) GetUserByPrincipalID(principalName string) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal v3.Principal) (*v3.Token, string, error) {
	panic("unimplemented")
}
