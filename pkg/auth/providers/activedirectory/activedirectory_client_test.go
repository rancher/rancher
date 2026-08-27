package activedirectory

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync/atomic"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/apiserver/pkg/apierror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	ldapFakes "github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	saUsername          = "FOO\\sa"
	saPassword          = "secret"
	userName            = "user"
	baseDN              = "ou=foo,dc=foo,dc=bar"
	userDN              = "cn=user," + baseDN
	groupDN             = "cn=group," + baseDN
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

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().CheckAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

	provider := adProvider{
		userMGR:  userManager,
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.Unauthorized, herr.Code)

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

		// Test-specific instance.
		userManager := userMocks.NewMockManager(ctrl)
		userManager.EXPECT().CheckAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()

		provider := provider
		provider.userMGR = userManager

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.PermissionDenied, herr.Code)

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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.MissingRequired, herr.Code)
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.Unauthorized, herr.Code)
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.ServerError, herr.Code)
	})

	t.Run("no user found", func(t *testing.T) {
		t.Parallel()

		ldapConn := &ldapFakes.FakeLdapConn{}
		provider := provider

		_, _, err := provider.loginUser(ldapConn, &credentials, &config)
		require.Error(t, err)

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.Unauthorized, herr.Code)
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.Unauthorized, herr.Code)
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.ServerError, herr.Code)
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.Unauthorized, herr.Code)
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

		herr, ok := err.(*apierror.APIError)
		require.True(t, ok)
		require.Equal(t, validation.Unauthorized, herr.Code)
	})
}

func TestLoginUserNTLMRejectsUPNBeforeAnyDirectoryIO(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		BindMechanism:          bindMechanismNTLM,
		TLS:                    true,
		ServiceAccountUsername: saUsername,
		ServiceAccountPassword: saPassword,
		UserObjectClass:        userObjectClassName,
		UserLoginAttribute:     "sAMAccountName",
		UserSearchBase:         baseDN,
	}

	ldapConn := &ldapFakes.FakeLdapConn{
		BindFunc: func(string, string) error {
			t.Fatal("no simple bind may be attempted")
			return nil
		},
		NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			t.Fatal("no NTLM bind may be attempted")
			return nil, nil
		},
		SearchFunc: func(*ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			t.Fatal("no search may be performed for an unsupported identity")
			return nil, nil
		},
	}

	provider := adProvider{tokenMGR: &tokens.Manager{}}
	credentials := v3.BasicLogin{Username: "alice@example.com", Password: userPassword}

	_, _, err := provider.loginUser(ldapConn, &credentials, &config)
	require.Error(t, err)

	herr, ok := err.(*apierror.APIError)
	require.True(t, ok)
	assert.Equal(t, validation.InvalidOption, herr.Code)
}

func TestLoginUserSimpleKeepsUPNBehaviour(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		ServiceAccountUsername: saUsername,
		ServiceAccountPassword: saPassword,
		UserObjectClass:        userObjectClassName,
		UserLoginAttribute:     "sAMAccountName",
		UserSearchBase:         baseDN,
	}

	var searched []string
	ldapConn := &ldapFakes.FakeLdapConn{
		BindFunc: func(string, string) error { return nil },
		SearchFunc: func(request *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			searched = append(searched, request.Filter)
			return &ldapv3.SearchResult{}, nil
		},
	}

	provider := adProvider{tokenMGR: &tokens.Manager{}}
	credentials := v3.BasicLogin{Username: "alice@example.com", Password: userPassword}

	// A UPN still reaches the search and fails there, exactly as before.
	_, _, err := provider.loginUser(ldapConn, &credentials, &config)
	require.Error(t, err)
	require.Len(t, searched, 1)
	assert.Contains(t, searched[0], "alice@example.com")
}

func TestLoginUserNTLMNeverPerformsASimpleBind(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		BindMechanism:               bindMechanismNTLM,
		TLS:                         true,
		DefaultLoginDomain:          "FOO",
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

	userSearchResult := &ldapv3.SearchResult{
		Entries: []*ldapv3.Entry{
			{
				DN: userDN,
				Attributes: []*ldapv3.EntryAttribute{
					{Name: ObjectClass, Values: []string{"top", "person", "user"}},
					{Name: "name", Values: []string{"user"}},
					{Name: "sAMAccountName", Values: []string{"user"}},
					{Name: "userAccountControl", Values: []string{"512"}},
				},
			},
		},
	}

	var ntlmBinds []ntlmIdentity
	ldapConn := &ldapFakes.FakeLdapConn{
		BindFunc: func(username, password string) error {
			t.Fatalf("simple bind reached with ntlm selected: %s", username)
			return nil
		},
		TLSConnectionStateFunc: tlsStateWithPeer(),
		NTLMChallengeBindFunc: func(request *ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			ntlmBinds = append(ntlmBinds, ntlmIdentity{Domain: request.Domain, Username: request.Username})
			return &ldapv3.NTLMBindResult{}, nil
		},
		SearchFunc: func(*ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return userSearchResult, nil
		},
		SearchWithPagingFunc: func(*ldapv3.SearchRequest, uint32) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{}, nil
		},
	}

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().CheckAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

	provider := adProvider{userMGR: userManager, tokenMGR: &tokens.Manager{}}
	credentials := v3.BasicLogin{Username: userName, Password: userPassword}

	_, _, err := provider.loginUser(ldapConn, &credentials, &config)
	require.NoError(t, err)

	require.Len(t, ntlmBinds, 2, "the service account bind and the user bind both go through NTLM")
	assert.Equal(t, ntlmIdentity{Domain: "FOO", Username: "sa"}, ntlmBinds[0])
	assert.Equal(t, ntlmIdentity{Domain: "FOO", Username: userName}, ntlmBinds[1])
}

func TestNTLMMechanismNeverPerformsASimpleBind(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		BindMechanism:               bindMechanismNTLM,
		TLS:                         true,
		DefaultLoginDomain:          "FOO",
		ServiceAccountUsername:      saUsername,
		ServiceAccountPassword:      saPassword,
		UserObjectClass:             userObjectClassName,
		UserLoginAttribute:          "sAMAccountName",
		UserNameAttribute:           "name",
		UserSearchBase:              baseDN,
		GroupDNAttribute:            "distinguishedName",
		GroupMemberMappingAttribute: "member",
		GroupMemberUserAttribute:    "distinguishedName",
		GroupNameAttribute:          "name",
		GroupObjectClass:            "group",
		GroupSearchAttribute:        "sAMAccountName",
		GroupSearchBase:             baseDN,
	}

	tests := []struct {
		name string
		call func(p *adProvider, conn ldapv3.Client, config *v3.ActiveDirectoryConfig)
	}{
		{
			name: "refetch group principals",
			call: func(p *adProvider, conn ldapv3.Client, config *v3.ActiveDirectoryConfig) {
				_, _ = p.refetchGroupPrincipalsOnConn(conn, config, UserScope+"://"+userDN)
			},
		},
		{
			name: "group principals from search",
			call: func(p *adProvider, conn ldapv3.Client, config *v3.ActiveDirectoryConfig) {
				_, _ = p.getGroupPrincipalsFromSearch(conn, config, baseDN, "(objectClass=group)", []string{userDN})
			},
		},
		{
			name: "get principal",
			call: func(p *adProvider, conn ldapv3.Client, config *v3.ActiveDirectoryConfig) {
				_, _ = p.getPrincipalOnConn(conn, userDN, UserScope, config)
			},
		},
		{
			name: "search ldap",
			call: func(p *adProvider, conn ldapv3.Client, config *v3.ActiveDirectoryConfig) {
				_, _ = p.searchLdap("(objectClass=person)", UserScope, config, conn)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var ntlmBinds []ntlmIdentity
			conn := &ldapFakes.FakeLdapConn{
				BindFunc: func(username, _ string) error {
					t.Fatalf("simple bind reached with ntlm selected: %s", username)
					return nil
				},
				TLSConnectionStateFunc: tlsStateWithPeer(),
				NTLMChallengeBindFunc: func(request *ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
					ntlmBinds = append(ntlmBinds, ntlmIdentity{Domain: request.Domain, Username: request.Username})
					return &ldapv3.NTLMBindResult{}, nil
				},
			}

			localConfig := config
			test.call(&adProvider{}, conn, &localConfig)

			require.Len(t, ntlmBinds, 1, "the service account bind goes through NTLM")
			assert.Equal(t, ntlmIdentity{Domain: "FOO", Username: "sa"}, ntlmBinds[0])
		})
	}
}

func TestLoginUserPreservesTheMissingServiceAccountPasswordCode(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		ServiceAccountUsername: saUsername,
		// ServiceAccountPassword deliberately empty.
		UserObjectClass:    userObjectClassName,
		UserLoginAttribute: "sAMAccountName",
		UserSearchBase:     baseDN,
	}

	ldapConn := &ldapFakes.FakeLdapConn{
		BindFunc: func(string, string) error {
			t.Fatal("no bind should be attempted without a service account password")
			return nil
		},
	}

	provider := adProvider{tokenMGR: &tokens.Manager{}}
	credentials := v3.BasicLogin{Username: userName, Password: userPassword}

	_, _, err := provider.loginUser(ldapConn, &credentials, &config)
	require.Error(t, err)

	herr, ok := err.(*apierror.APIError)
	require.True(t, ok)
	assert.Equal(t, validation.MissingRequired, herr.Code,
		"the classifier must not flatten an APIError the bind helpers already produced")
}

func TestLoginUserNTLMSurfacesAChannelBindingRejection(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		BindMechanism:          bindMechanismNTLM,
		TLS:                    true,
		DefaultLoginDomain:     "FOO",
		ServiceAccountUsername: saUsername,
		ServiceAccountPassword: saPassword,
		UserObjectClass:        userObjectClassName,
		UserLoginAttribute:     "sAMAccountName",
		UserSearchBase:         baseDN,
	}

	ldapConn := &ldapFakes.FakeLdapConn{
		TLSConnectionStateFunc: tlsStateWithPeer(),
		NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			return nil, ldapError(ldapv3.LDAPResultInvalidCredentials,
				"comment: AcceptSecurityContext error, data 80090346, v4563")
		},
	}

	provider := adProvider{tokenMGR: &tokens.Manager{}}
	credentials := v3.BasicLogin{Username: userName, Password: userPassword}

	_, _, err := provider.loginUser(ldapConn, &credentials, &config)
	require.Error(t, err)

	herr, ok := err.(*apierror.APIError)
	require.True(t, ok)
	assert.Equal(t, validation.ServerError, herr.Code)
	assert.Contains(t, herr.Message, "80090346")
	assert.Contains(t, herr.Message, "channel binding")
}

// bindFailingProvider returns a provider whose connection is real enough to
// bind and search, and whose bind fails with the given diagnostic.
func bindFailingProvider(t *testing.T, diagnostic string, enabled bool) (*adProvider, *atomic.Bool) {
	t.Helper()

	var closed atomic.Bool
	conn := &ldapFakes.FakeLdapConn{
		BindFunc: func(string, string) error {
			return ldapError(ldapv3.LDAPResultInvalidCredentials, diagnostic)
		},
		SearchFunc: func(*ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			t.Fatal("search must not run after a failed bind")
			return nil, nil
		},
		CloseFunc: func() error { closed.Store(true); return nil },
	}

	// No ownership assertion here. This helper serves two kinds of test, and
	// only one of them owns the connection: a provider entry point defers
	// Close, while the ...OnConn helpers deliberately do not close a
	// connection they were handed. An unconditional cleanup assertion fails
	// every direct-helper test even when its subject passes.
	//
	// Callers that do own the connection assert `closed` themselves.

	return &adProvider{
		loadConfig: func() (*v3.ActiveDirectoryConfig, *x509.CertPool, error) {
			return &v3.ActiveDirectoryConfig{
				AuthConfig:             v3.AuthConfig{Enabled: enabled},
				ServiceAccountUsername: saUsername,
				ServiceAccountPassword: saPassword,
				UserSearchBase:         baseDN,
				UserObjectClass:        userObjectClassName,
				UserLoginAttribute:     "sAMAccountName",
				UserNameAttribute:      "name",
				GroupObjectClass:       "group",
				GroupNameAttribute:     "name",
				GroupSearchAttribute:   "sAMAccountName",
				GroupDNAttribute:       "distinguishedName",
			}, nil, nil
		},
		dial: func(*v3.ActiveDirectoryConfig, *x509.CertPool) (ldapv3.Client, error) { return conn, nil },
	}, &closed
}

func TestSearchPrincipalsSurfacesNonCredentialBindFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		diagnostic string
		wantInMsg  string
	}{
		{name: "bad bindings", diagnostic: capturedBadBindings, wantInMsg: "channel binding"},
		{name: "malformed token", diagnostic: capturedBadToken, wantInMsg: "malformed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider, closed := bindFailingProvider(t, test.diagnostic, true)

			got, err := provider.SearchPrincipals("alice", "user", &v3.Token{})

			require.Error(t, err, "the mapped error must reach the caller, not be swallowed")
			assert.True(t, closed.Load(), "SearchPrincipals owns the connection and must close it")
			assert.Empty(t, got)
			assert.Contains(t, err.Error(), test.wantInMsg)

			herr, ok := err.(*apierror.APIError)
			require.True(t, ok)
			assert.Equal(t, validation.ServerError, herr.Code)
		})
	}
}

func TestSearchPrincipalsStillSwallowsOrdinaryFailures(t *testing.T) {
	t.Parallel()

	// A wrong service-account password must keep returning an empty result and
	// a nil error. Callers depend on principal search degrading rather than
	// erroring, and only the two non-credential classes may change that.
	provider, closed := bindFailingProvider(t, typicalWrongPassword, true)

	got, err := provider.SearchPrincipals("alice", "user", &v3.Token{})

	assert.NoError(t, err)
	assert.Empty(t, got)
	assert.True(t, closed.Load())
}

func TestGroupPrincipalsFallbackDistinguishesFailureKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		diagnostic   string
		wantFallback bool   // degrade to memberOf-derived principals
		wantInMsg    string // when not degrading
	}{
		{
			name:         "rotated password still degrades",
			diagnostic:   typicalWrongPassword,
			wantFallback: true,
		},
		{
			name:       "bad bindings surfaces",
			diagnostic: capturedBadBindings,
			wantInMsg:  "channel binding",
		},
		{
			name:       "malformed token surfaces",
			diagnostic: capturedBadToken,
			wantInMsg:  "malformed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider, _ := bindFailingProvider(t, test.diagnostic, true)
			config, _, err := provider.configOrDefault()
			require.NoError(t, err)
			conn, err := provider.ldapConnectionOrDefault(config, nil)
			require.NoError(t, err)

			// getGroupPrincipalsFromSearch is handed a connection it does not
			// own, so closing it is this test's job.
			defer func() { _ = conn.Close() }()

			got, err := provider.getGroupPrincipalsFromSearch(
				conn, config, baseDN, "(objectClass=group)", []string{groupDN})

			if test.wantFallback {
				require.NoError(t, err, "a rotated password must still degrade")
				require.Len(t, got, 1, "the fallback returns principals derived from memberOf")
				assert.Equal(t, GroupScope+"://"+groupDN, got[0].Name)
				return
			}

			require.Error(t, err)
			assert.Empty(t, got, "partial group data must not be returned for a non-credential failure")
			assert.Contains(t, err.Error(), test.wantInMsg)
		})
	}
}

func TestGetPrincipalFallbackDistinguishesFailureKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		diagnostic   string
		wantFallback bool
		wantInMsg    string
	}{
		{
			name:         "rotated password still degrades",
			diagnostic:   typicalWrongPassword,
			wantFallback: true,
		},
		{
			name:       "bad bindings surfaces",
			diagnostic: capturedBadBindings,
			wantInMsg:  "channel binding",
		},
		{
			name:       "malformed token surfaces",
			diagnostic: capturedBadToken,
			wantInMsg:  "malformed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider, _ := bindFailingProvider(t, test.diagnostic, true)
			config, _, err := provider.configOrDefault()
			require.NoError(t, err)
			conn, err := provider.ldapConnectionOrDefault(config, nil)
			require.NoError(t, err)

			// Same ownership rule as site 4: the ...OnConn helper does not
			// close a connection it was given.
			defer func() { _ = conn.Close() }()

			got, err := provider.getPrincipalOnConn(conn, userDN, UserScope, config)

			if test.wantFallback {
				// Site 5 degrades differently from site 4: it returns a single
				// principal formed from the DN, not principals derived from
				// memberOf.
				require.NoError(t, err, "a rotated password must still degrade")
				require.NotNil(t, got)
				assert.Equal(t, UserScope+"://"+userDN, got.Name)
				assert.Equal(t, userDN, got.LoginName)
				return
			}

			require.Error(t, err)
			assert.Nil(t, got, "a DN-formed principal must not stand in for a non-credential failure")
			assert.Contains(t, err.Error(), test.wantInMsg)
		})
	}
}

func TestLoginUserNTLMSurfacesABindPreconditionFailure(t *testing.T) {
	t.Parallel()

	config := v3.ActiveDirectoryConfig{
		BindMechanism:          bindMechanismNTLM,
		TLS:                    true,
		DefaultLoginDomain:     "FOO",
		ServiceAccountUsername: saUsername,
		ServiceAccountPassword: saPassword,
		UserObjectClass:        userObjectClassName,
		UserLoginAttribute:     "sAMAccountName",
		UserSearchBase:         baseDN,
	}

	// The TLS state disappears between the service-account bind and the user
	// bind, so the user bind fails one of the bind helper's own preconditions.
	// The APIError it returns must reach the caller with its message intact
	// rather than be re-wrapped into the generic server error.
	var tlsStateCalls atomic.Int32
	ldapConn := &ldapFakes.FakeLdapConn{
		TLSConnectionStateFunc: func() (tls.ConnectionState, bool) {
			if tlsStateCalls.Add(1) == 1 {
				return tlsStateWithPeer()()
			}
			return tls.ConnectionState{}, false
		},
		NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			return &ldapv3.NTLMBindResult{}, nil
		},
		SearchFunc: func(*ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{
					{
						DN: userDN,
						Attributes: []*ldapv3.EntryAttribute{
							{Name: ObjectClass, Values: []string{"top", "person", "user"}},
							{Name: "sAMAccountName", Values: []string{userName}},
						},
					},
				},
			}, nil
		},
	}

	provider := adProvider{tokenMGR: &tokens.Manager{}}
	credentials := v3.BasicLogin{Username: userName, Password: userPassword}

	_, _, err := provider.loginUser(ldapConn, &credentials, &config)
	require.Error(t, err)

	herr, ok := err.(*apierror.APIError)
	require.True(t, ok)
	assert.Equal(t, validation.ServerError, herr.Code)
	assert.Contains(t, herr.Message, "TLS")
	assert.NotEqual(t, "server error while authenticating", herr.Message,
		"the bind helper's own diagnostic must not be flattened to the generic message")
}
