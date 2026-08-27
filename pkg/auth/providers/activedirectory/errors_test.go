package activedirectory

import (
	"errors"
	"fmt"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ldapError builds the error shape go-ldap returns for a failed bind. The
// diagnostic text lives in Err; Error() renders it as
// `LDAP Result Code 49 "Invalid Credentials": <diagnostic>`.
func ldapError(code uint16, diagnostic string) error {
	return &ldapv3.Error{ResultCode: code, Err: errors.New(diagnostic)}
}

// The two diagnostics below are verbatim captures from a live domain
// controller on 2026-08-26. Synthetic strings are what let the previous
// revision ship an unreachable mapping: it tested `data 80090308`, a shape AD
// never emits, while the real message carries `data 57`.
const (
	capturedBadBindings = "80090346: LdapErr: DSID-0C09059A, comment: AcceptSecurityContext error, data 80090346, v4563"
	capturedBadToken    = "80090308: LdapErr: DSID-0C09089F, comment: AcceptSecurityContext error, data 57, v4563"
	// Not captured here, but the shape AD uses for a wrong password. It shares
	// its security status with capturedBadToken, which is why the pair matters.
	typicalWrongPassword = "80090308: LdapErr: DSID-0C0903A9, comment: AcceptSecurityContext error, data 52e, v4563"
)

func TestParseADDiagnostic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantSec    string
		wantData   string
		wantParsed bool
	}{
		{
			name:    "bad bindings, both fields the same",
			err:     ldapError(ldapv3.LDAPResultInvalidCredentials, capturedBadBindings),
			wantSec: "80090346", wantData: "80090346", wantParsed: true,
		},
		{
			name:    "malformed token, fields differ",
			err:     ldapError(ldapv3.LDAPResultInvalidCredentials, capturedBadToken),
			wantSec: "80090308", wantData: "57", wantParsed: true,
		},
		{
			name:    "wrong password shares the security status",
			err:     ldapError(ldapv3.LDAPResultInvalidCredentials, typicalWrongPassword),
			wantSec: "80090308", wantData: "52e", wantParsed: true,
		},
		{
			name: "no diagnostic fields",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, "invalid credentials"),
		},
		{
			name: "the word data without a hex value",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, "the data was rejected"),
		},
		{name: "not an ldap error", err: errors.New("connection refused")},
		{name: "nil", err: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseADDiagnostic(test.err)
			assert.Equal(t, test.wantParsed, ok)
			assert.Equal(t, test.wantSec, got.SecurityStatus)
			assert.Equal(t, test.wantData, got.Data)
		})
	}
}

func TestClassifyBindFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bindFailureKind
	}{
		{
			name: "bad bindings",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, capturedBadBindings),
			want: bindFailureBadBindings,
		},
		{
			name: "malformed token",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, capturedBadToken),
			want: bindFailureMalformedToken,
		},
		{
			// The security status alone would misclassify this as malformed,
			// reporting a wrong password as a Rancher defect.
			name: "wrong password is not a construction failure",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, typicalWrongPassword),
			want: bindFailureNone,
		},
		{
			name: "data 57 without the matching security status",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, "8009030c: LdapErr: comment: AcceptSecurityContext error, data 57, v4563"),
			want: bindFailureNone,
		},
		{
			// The asymmetry, pinned: bad bindings is keyed on the data value
			// alone, so a different leading status must not change the result.
			name: "bad bindings under a different security status still classifies",
			err:  ldapError(ldapv3.LDAPResultInvalidCredentials, "8009030c: LdapErr: comment: AcceptSecurityContext error, data 80090346, v4563"),
			want: bindFailureBadBindings,
		},
		{name: "plain error", err: errors.New("dial tcp: connection refused"), want: bindFailureNone},
		{name: "nil", err: nil, want: bindFailureNone},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, classifyBindFailure(test.err))
		})
	}
}

func TestMapBindError(t *testing.T) {
	t.Parallel()

	t.Run("bad bindings explains the likely causes", func(t *testing.T) {
		t.Parallel()

		in := ldapError(ldapv3.LDAPResultInvalidCredentials, capturedBadBindings)

		got := mapBindError(in)
		require.Error(t, got)
		assert.Contains(t, got.Error(), "channel binding")
		assert.Contains(t, got.Error(), "80090346")
		assert.Contains(t, got.Error(), "proxy")
		assert.NotContains(t, got.Error(), "server name",
			"TLS verification already checks the certificate name, so suggesting it misdirects the operator")
		assert.ErrorIs(t, got, in)
	})

	t.Run("malformed token is named as a Rancher defect", func(t *testing.T) {
		t.Parallel()

		in := ldapError(ldapv3.LDAPResultInvalidCredentials, capturedBadToken)

		got := mapBindError(in)
		require.Error(t, got)
		assert.Contains(t, got.Error(), "malformed")
		assert.Contains(t, got.Error(), "80090308")
		assert.NotContains(t, got.Error(), "channel binding token",
			"a malformed message is not a channel binding rejection and must not read as one")
		assert.ErrorIs(t, got, in)
	})

	t.Run("wrong password is untouched", func(t *testing.T) {
		t.Parallel()

		in := ldapError(ldapv3.LDAPResultInvalidCredentials, typicalWrongPassword)

		assert.Equal(t, in, mapBindError(in), "lockout and negative authentication behaviour is unchanged")
		assert.True(t, ldapv3.IsErrorWithCode(mapBindError(in), ldapv3.LDAPResultInvalidCredentials))
	})

	t.Run("non ldap errors pass through", func(t *testing.T) {
		t.Parallel()

		in := fmt.Errorf("dial tcp: connection refused")
		assert.Equal(t, in, mapBindError(in))
	})

	t.Run("nil stays nil", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, mapBindError(nil))
	})
}
