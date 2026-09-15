package activedirectory

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	ldapFakes "github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBindConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  v3.ActiveDirectoryConfig
		wantErr bool
	}{
		{
			name:   "empty mechanism is simple",
			config: v3.ActiveDirectoryConfig{},
		},
		{
			name:   "simple over plaintext",
			config: v3.ActiveDirectoryConfig{BindMechanism: "simple"},
		},
		{
			name:   "ntlm over tls",
			config: v3.ActiveDirectoryConfig{BindMechanism: "ntlm", TLS: true},
		},
		{
			name:   "ntlm over starttls",
			config: v3.ActiveDirectoryConfig{BindMechanism: "ntlm", StartTLS: true},
		},
		{
			name:    "ntlm over plaintext",
			config:  v3.ActiveDirectoryConfig{BindMechanism: "ntlm"},
			wantErr: true,
		},
		{
			name:    "kerberos is reserved but not accepted",
			config:  v3.ActiveDirectoryConfig{BindMechanism: "kerberos", TLS: true},
			wantErr: true,
		},
		{
			name:    "unknown mechanism",
			config:  v3.ActiveDirectoryConfig{BindMechanism: "gssapi", TLS: true},
			wantErr: true,
		},
		{
			name:    "mechanism is case sensitive",
			config:  v3.ActiveDirectoryConfig{BindMechanism: "NTLM", TLS: true},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateBindConfiguration(&test.config)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDecodeActiveDirectoryConfigRejectsBadMechanism(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mechanism string
		tls       bool
		wantErr   bool
	}{
		{name: "unknown mechanism", mechanism: "gssapi", tls: true, wantErr: true},
		{name: "ntlm over plaintext", mechanism: bindMechanismNTLM, wantErr: true},
		{name: "kerberos reserved", mechanism: bindMechanismKerberos, tls: true, wantErr: true},
		{name: "ntlm over tls", mechanism: bindMechanismNTLM, tls: true},
		{name: "empty is simple", mechanism: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := (&adProvider{}).decodeActiveDirectoryConfig(map[string]any{
				"metadata":      map[string]any{"name": "activedirectory"},
				"bindMechanism": test.mechanism,
				"tls":           test.tls,
				"servers":       []any{"dc1.example.com"},
			})

			if !test.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, errBindConfiguration)
			assert.Contains(t, err.Error(), "bindMechanism")
		})
	}
}

// badConfigProvider returns a provider whose config load fails with the given
// error, so the three swallowing callers can be driven directly.
func badConfigProvider(err error) *adProvider {
	return &adProvider{
		loadConfig: func() (*v3.ActiveDirectoryConfig, *x509.CertPool, error) { return nil, nil, err },
		dial: func(*v3.ActiveDirectoryConfig, *x509.CertPool) (ldapv3.Client, error) {
			panic("dial must not be reached when the config failed to load")
		},
	}
}

func TestStoredConfigCallersSurfaceBindConfigurationErrors(t *testing.T) {
	t.Parallel()

	loadErr := fmt.Errorf("%w: invalid bindMechanism %q", errBindConfiguration, "gssapi")

	t.Run("AuthenticateUser", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := badConfigProvider(loadErr).AuthenticateUser(nil, nil,
			&v3.BasicLogin{Username: userName, Password: userPassword})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "bindMechanism")
		assert.NotContains(t, err.Error(), "can't find authprovider",
			"a bind configuration fault must not be reported as a missing auth provider")
	})

	t.Run("SearchPrincipals", func(t *testing.T) {
		t.Parallel()

		got, err := badConfigProvider(loadErr).SearchPrincipals("alice", "user", &v3.Token{})

		require.Error(t, err, "previously returned a nil error and an empty slice")
		assert.Empty(t, got)
		assert.Contains(t, err.Error(), "bindMechanism")
	})

	t.Run("GetPrincipal", func(t *testing.T) {
		t.Parallel()

		_, err := badConfigProvider(loadErr).GetPrincipal(UserScope+"://"+userDN, &v3.Token{})

		require.Error(t, err, "previously returned a nil error and a zero principal")
		assert.Contains(t, err.Error(), "bindMechanism")
	})
}

func TestStoredConfigCallersStillSwallowUnrelatedFailures(t *testing.T) {
	t.Parallel()

	// Anything that is not errBindConfiguration keeps the historic behaviour.
	// This is what stops the change becoming a wider behavioural one than
	// intended, and it is the test most likely to fail if the sentinel check
	// is written as a catch-all.
	loadErr := errors.New("failed to retrieve ActiveDirectoryConfig: connection refused")

	t.Run("AuthenticateUser still reports the generic message", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := badConfigProvider(loadErr).AuthenticateUser(nil, nil,
			&v3.BasicLogin{Username: userName, Password: userPassword})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "can't find authprovider")
	})

	t.Run("SearchPrincipals still returns empty and nil", func(t *testing.T) {
		t.Parallel()

		got, err := badConfigProvider(loadErr).SearchPrincipals("alice", "user", &v3.Token{})

		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("GetPrincipal still returns zero and nil", func(t *testing.T) {
		t.Parallel()

		got, err := badConfigProvider(loadErr).GetPrincipal(UserScope+"://"+userDN, &v3.Token{})

		assert.NoError(t, err)
		assert.Empty(t, got.Name)
	})
}

func TestConnectForTestAndApplyValidatesBeforeDialling(t *testing.T) {
	t.Parallel()

	var dialled atomic.Bool
	provider := &adProvider{
		dial: func(*v3.ActiveDirectoryConfig, *x509.CertPool) (ldapv3.Client, error) {
			dialled.Store(true)
			return nil, errors.New("dial must not be reached")
		},
	}

	_, err := provider.connectForTestAndApply(&v3.ActiveDirectoryTestAndApplyInput{
		Username: userName,
		Password: userPassword,
		ActiveDirectoryConfig: v3.ActiveDirectoryConfig{
			BindMechanism: bindMechanismNTLM, // no TLS, no StartTLS
			Servers:       []string{"dc1.example.com"},
		},
	})

	require.Error(t, err)
	herr, ok := err.(*httperror.APIError)
	require.True(t, ok)
	assert.Equal(t, httperror.InvalidBodyContent, herr.Code)
	assert.False(t, dialled.Load(), "validation must run before the connection is opened")
}

func TestConnectForTestAndApplyDialsWhenValid(t *testing.T) {
	t.Parallel()

	// The mirror case: without it, a validator that rejected everything would
	// pass the test above.
	var dialled atomic.Bool
	provider := &adProvider{
		dial: func(*v3.ActiveDirectoryConfig, *x509.CertPool) (ldapv3.Client, error) {
			dialled.Store(true)
			return &ldapFakes.FakeLdapConn{}, nil
		},
	}

	_, err := provider.connectForTestAndApply(&v3.ActiveDirectoryTestAndApplyInput{
		Username: userName,
		Password: userPassword,
		ActiveDirectoryConfig: v3.ActiveDirectoryConfig{
			BindMechanism: bindMechanismNTLM,
			TLS:           true,
			Servers:       []string{"dc1.example.com"},
		},
	})

	require.NoError(t, err)
	assert.True(t, dialled.Load())
}

func TestSplitNTLMIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		username      string
		defaultDomain string
		wantDomain    string
		wantUser      string
		wantErr       bool
	}{
		{
			name:       "domain qualified",
			username:   `FOO\alice`,
			wantDomain: "FOO",
			wantUser:   "alice",
		},
		{
			name:          "domain qualified ignores the default domain",
			username:      `FOO\alice`,
			defaultDomain: "BAR",
			wantDomain:    "FOO",
			wantUser:      "alice",
		},
		{
			name:          "bare name uses the default domain",
			username:      "alice",
			defaultDomain: "FOO",
			wantDomain:    "FOO",
			wantUser:      "alice",
		},
		{
			name:     "bare name without a default domain",
			username: "alice",
			wantErr:  true,
		},
		{name: "empty", username: "", defaultDomain: "FOO", wantErr: true},
		{name: "empty domain", username: `\alice`, defaultDomain: "FOO", wantErr: true},
		{name: "empty user", username: `FOO\`, wantErr: true},
		{name: "only a backslash", username: `\`, wantErr: true},
		{name: "two backslashes", username: `A\B\alice`, wantErr: true},
		{name: "upn", username: "alice@example.com", defaultDomain: "FOO", wantErr: true},
		{name: "upn with a domain prefix", username: `FOO\alice@example.com`, wantErr: true},
		{name: "distinguished name", username: "cn=alice,ou=foo,dc=example,dc=com", defaultDomain: "FOO", wantErr: true},
		{name: "whitespace only", username: "   ", defaultDomain: "FOO", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			domain, user, err := splitNTLMIdentity(test.username, test.defaultDomain)
			if test.wantErr {
				require.Error(t, err)

				herr, ok := err.(*apierror.APIError)
				require.True(t, ok, "callers map this to an operator readable API error")
				assert.Equal(t, validation.InvalidOption, herr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantDomain, domain)
			assert.Equal(t, test.wantUser, user)
		})
	}
}

// testPeerCertificate returns a certificate a channel binding token can be
// derived from.
func testPeerCertificate() *x509.Certificate {
	return &x509.Certificate{Raw: []byte("test-der-bytes"), SignatureAlgorithm: x509.SHA256WithRSA}
}

func tlsStateWithPeer() func() (tls.ConnectionState, bool) {
	return func() (tls.ConnectionState, bool) {
		return tls.ConnectionState{PeerCertificates: []*x509.Certificate{testPeerCertificate()}}, true
	}
}

func TestBindAsSimpleMechanism(t *testing.T) {
	t.Parallel()

	var bound []v3.BasicLogin
	conn := &ldapFakes.FakeLdapConn{
		BindFunc: func(username, password string) error {
			bound = append(bound, v3.BasicLogin{Username: username, Password: password})
			return nil
		},
		NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			t.Fatal("an NTLM bind must not happen for the simple mechanism")
			return nil, nil
		},
	}

	config := &v3.ActiveDirectoryConfig{DefaultLoginDomain: "FOO"}
	provider := &adProvider{}

	require.NoError(t, provider.bindAs(conn, config, nil, "alice", "secret"))
	require.Len(t, bound, 1)
	assert.Equal(t, v3.BasicLogin{Username: `FOO\alice`, Password: "secret"}, bound[0],
		"the simple path keeps using GetUserExternalID verbatim")
}

func TestBindAsNTLMMechanism(t *testing.T) {
	t.Parallel()

	var request *ldapv3.NTLMBindRequest
	conn := &ldapFakes.FakeLdapConn{
		BindFunc: func(string, string) error {
			t.Fatal("a simple bind must never happen once ntlm is selected")
			return nil
		},
		TLSConnectionStateFunc: tlsStateWithPeer(),
		NTLMChallengeBindFunc: func(r *ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			request = r
			return &ldapv3.NTLMBindResult{}, nil
		},
	}

	config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM, TLS: true, DefaultLoginDomain: "FOO"}
	provider := &adProvider{}

	require.NoError(t, provider.bindAs(conn, config, nil, "alice", "secret"))
	require.NotNil(t, request)
	assert.Equal(t, "FOO", request.Domain)
	assert.Equal(t, "alice", request.Username)
	assert.Equal(t, "secret", request.Password)
	assert.Empty(t, request.Hash, "pass the hash is never exposed")
	assert.NotNil(t, request.Negotiator)
}

func TestBindAsNTLMUsesThePreflightIdentity(t *testing.T) {
	t.Parallel()

	var request *ldapv3.NTLMBindRequest
	conn := &ldapFakes.FakeLdapConn{
		BindFunc:               func(string, string) error { t.Fatal("no simple bind"); return nil },
		TLSConnectionStateFunc: tlsStateWithPeer(),
		NTLMChallengeBindFunc: func(r *ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
			request = r
			return &ldapv3.NTLMBindResult{}, nil
		},
	}

	config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM, TLS: true, DefaultLoginDomain: "IGNORED"}
	provider := &adProvider{}
	identity := &ntlmIdentity{Domain: "BAR", Username: "bob"}

	require.NoError(t, provider.bindAs(conn, config, identity, "whatever-raw-value", "secret"))
	require.NotNil(t, request)
	assert.Equal(t, "BAR", request.Domain)
	assert.Equal(t, "bob", request.Username, "the resolved identity is used, the raw value is not re-parsed")
}

func TestBindAsRejects(t *testing.T) {
	t.Parallel()

	failIfBound := func(t *testing.T) *ldapFakes.FakeLdapConn {
		t.Helper()
		return &ldapFakes.FakeLdapConn{
			BindFunc: func(string, string) error {
				t.Fatal("no bind of any kind should be attempted")
				return nil
			},
			NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
				t.Fatal("no bind of any kind should be attempted")
				return nil, nil
			},
		}
	}

	t.Run("unknown mechanism", func(t *testing.T) {
		t.Parallel()

		provider := &adProvider{}
		config := &v3.ActiveDirectoryConfig{BindMechanism: "gssapi", TLS: true}
		require.Error(t, provider.bindAs(failIfBound(t), config, nil, "alice", "secret"))
	})

	t.Run("ntlm over plaintext", func(t *testing.T) {
		t.Parallel()

		provider := &adProvider{}
		config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM}
		require.Error(t, provider.bindAs(failIfBound(t), config, nil, "alice", "secret"))
	})

	t.Run("no tls connection state", func(t *testing.T) {
		t.Parallel()

		conn := &ldapFakes.FakeLdapConn{
			BindFunc: func(string, string) error { t.Fatal("no simple bind fallback"); return nil },
			TLSConnectionStateFunc: func() (tls.ConnectionState, bool) {
				return tls.ConnectionState{}, false
			},
			NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
				t.Fatal("an unbound NTLM bind must not be attempted")
				return nil, nil
			},
		}
		provider := &adProvider{}
		config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM, TLS: true}
		require.Error(t, provider.bindAs(conn, config, nil, "alice", "secret"))
	})

	t.Run("no peer certificate", func(t *testing.T) {
		t.Parallel()

		conn := &ldapFakes.FakeLdapConn{
			BindFunc: func(string, string) error { t.Fatal("no simple bind fallback"); return nil },
			TLSConnectionStateFunc: func() (tls.ConnectionState, bool) {
				return tls.ConnectionState{}, true
			},
			NTLMChallengeBindFunc: func(*ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error) {
				t.Fatal("an unbound NTLM bind must not be attempted")
				return nil, nil
			},
		}
		provider := &adProvider{}
		config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM, TLS: true}
		require.Error(t, provider.bindAs(conn, config, nil, "alice", "secret"))
	})

	t.Run("connection cannot do an ntlm challenge bind", func(t *testing.T) {
		t.Parallel()

		provider := &adProvider{}
		config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM, TLS: true, DefaultLoginDomain: "FOO"}

		conn := &nonNTLMConn{Client: failIfBound(t)}
		require.Error(t, provider.bindAs(conn, config, nil, "alice", "secret"))
	})

	t.Run("upn is rejected before any bind", func(t *testing.T) {
		t.Parallel()

		provider := &adProvider{}
		config := &v3.ActiveDirectoryConfig{BindMechanism: bindMechanismNTLM, TLS: true}
		conn := failIfBound(t)
		conn.TLSConnectionStateFunc = tlsStateWithPeer()

		require.Error(t, provider.bindAs(conn, config, nil, "alice@example.com", "secret"))
	})
}

// nonNTLMConn stands in for a connection that cannot carry an NTLM bind.
// Embedding the ldapv3.Client interface rather than the fake struct promotes
// only the interface's methods, and NTLMChallengeBind is not one of them, so
// the value satisfies ldapv3.Client but fails the ntlmChallengeBinder
// assertion.
type nonNTLMConn struct {
	ldapv3.Client
}

var (
	_ ldapv3.Client = &nonNTLMConn{}
	_ ldapv3.Client = &ldapFakes.FakeLdapConn{}
)

func TestBindServiceAccountRequiresAPassword(t *testing.T) {
	t.Parallel()

	provider := &adProvider{}
	config := &v3.ActiveDirectoryConfig{ServiceAccountUsername: `FOO\sa`}

	conn := &ldapFakes.FakeLdapConn{
		BindFunc: func(string, string) error {
			t.Fatal("an empty service account password must not reach a bind")
			return nil
		},
	}

	err := provider.bindServiceAccount(conn, config)
	require.Error(t, err)

	herr, ok := err.(*apierror.APIError)
	require.True(t, ok)
	assert.Equal(t, validation.MissingRequired, herr.Code)
}
