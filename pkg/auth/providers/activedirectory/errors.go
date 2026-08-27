package activedirectory

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

// AD encodes two independent status fields in the diagnostic message of an
// invalidCredentials result, and they are not the same value:
//
//	80090346: LdapErr: DSID-0C09059A, comment: AcceptSecurityContext error, data 80090346, v4563
//	80090308: LdapErr: DSID-0C09089F, comment: AcceptSecurityContext error, data 57, v4563
//
// Both were captured from a live domain controller. The leading field is the
// SSPI security status; the `data` token is AD's own sub-status. In the first
// they coincide, in the second they do not — which is why parsing only one of
// them cannot classify both.
var (
	securityStatusPattern = regexp.MustCompile(`(?i)^\s*([0-9a-f]{8}):`)
	dataStatusPattern     = regexp.MustCompile(`(?i)\bdata\s+([0-9a-f]{1,8})\s*(?:,|$)`)
)

const (
	// dataBadBindings is SEC_E_BAD_BINDINGS as AD reports it in the data
	// token: the bind carried no channel binding token, or one that did not
	// match the TLS channel it arrived on.
	dataBadBindings = "80090346"

	// dataInvalidParameter is ERROR_INVALID_PARAMETER. Paired with a
	// SEC_E_INVALID_TOKEN security status it means the AUTHENTICATE message
	// was refused as malformed, without the binding being evaluated.
	dataInvalidParameter = "57"

	// secInvalidToken is SEC_E_INVALID_TOKEN. It is NOT sufficient on its own:
	// AD also reports it for ordinary credential failures such as data 52e
	// (wrong password) and data 533 (account disabled). Only the pair
	// identifies a malformed message.
	secInvalidToken = "80090308"
)

// adDiagnostic holds the two status fields parsed out of an AD diagnostic.
// Either may be absent.
type adDiagnostic struct {
	SecurityStatus string
	Data           string
}

// parseADDiagnostic extracts both status fields from an LDAP error.
func parseADDiagnostic(err error) (adDiagnostic, bool) {
	var ldapErr *ldapv3.Error
	if !errors.As(err, &ldapErr) {
		return adDiagnostic{}, false
	}

	// Match against the underlying diagnostic, where the security status is
	// still leading: ldapv3.Error.Error() prefixes its own text, and it also
	// renders through Err unconditionally, so with a nil Err there is both
	// nothing to parse and nothing safe to call.
	if ldapErr.Err == nil {
		return adDiagnostic{}, false
	}
	diagnostic := ldapErr.Err.Error()

	var out adDiagnostic
	if m := securityStatusPattern.FindStringSubmatch(diagnostic); m != nil {
		out.SecurityStatus = strings.ToLower(m[1])
	}
	if m := dataStatusPattern.FindStringSubmatch(diagnostic); m != nil {
		out.Data = strings.ToLower(m[1])
	}
	return out, out.SecurityStatus != "" || out.Data != ""
}

// bindFailureKind classifies a bind failure that is not a credential problem.
type bindFailureKind int

const (
	bindFailureNone bindFailureKind = iota
	bindFailureBadBindings
	bindFailureMalformedToken
)

// classifyBindFailure reports whether a bind error is one of the two
// non-credential failures this feature can produce.
//
// Both arrive as LDAP result code 49, indistinguishable from a wrong password
// unless the diagnostic is parsed. Every branch that treats code 49 as a
// credential problem — the Unauthorized mapping, and the config.Enabled
// password-rotation fallbacks — must consult this first.
func classifyBindFailure(err error) bindFailureKind {
	d, ok := parseADDiagnostic(err)
	if !ok {
		return bindFailureNone
	}

	switch {
	case d.Data == dataBadBindings:
		// Data alone, deliberately. An eight-hex SEC_E_* value in the data
		// field is self-identifying — AD uses short sub-statuses (52e, 533,
		// 57) for everything else — so the security status adds no
		// discrimination here, and requiring it would break against any DC
		// version reporting this sub-status under a different leading value.
		// The malformed-token case below is the opposite: two digits, no such
		// guarantee, so it needs both fields.
		return bindFailureBadBindings
	case d.Data == dataInvalidParameter && d.SecurityStatus == secInvalidToken:
		return bindFailureMalformedToken
	default:
		// Everything else, including data 52e and the rest of the AD
		// credential sub-statuses, keeps its existing handling.
		return bindFailureNone
	}
}

// mapBindError turns a non-credential bind failure into a message an operator
// can act on, and leaves every other error exactly as it was so existing
// lockout and invalid-credential handling is unaffected.
func mapBindError(err error) error {
	if err == nil {
		return nil
	}

	switch classifyBindFailure(err) {
	case bindFailureBadBindings:
		// A bad-bindings result on a TLS channel Rancher already verified most
		// often means the channel is being terminated somewhere in between.
		return fmt.Errorf("the domain controller rejected the channel binding token (%s, SEC_E_BAD_BINDINGS). "+
			"Likely causes, in order: the bind carried no channel binding token or a malformed one; "+
			"the token was derived with the wrong certificate hash algorithm; "+
			"or a proxy or appliance is terminating and re-originating TLS between Rancher and the domain controller: %w",
			dataBadBindings, err)

	case bindFailureMalformedToken:
		return fmt.Errorf("the domain controller rejected the NTLM token as malformed "+
			"(SEC_E_INVALID_TOKEN %s, data %s). This indicates a defect in how Rancher builds the "+
			"authenticate message — not a credential, certificate or configuration problem: %w",
			secInvalidToken, dataInvalidParameter, err)

	default:
		return err
	}
}
