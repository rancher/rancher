package activedirectory

import (
	"crypto/x509"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	ldapFakes "github.com/rancher/rancher/pkg/auth/providers/common/ldap"
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
