package activedirectory

import (
	"errors"
	"fmt"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/apiserver/pkg/apierror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
)

const (
	bindMechanismSimple   = "simple"
	bindMechanismNTLM     = "ntlm"
	bindMechanismKerberos = "kerberos"
)

// validateBindConfiguration reports whether config selects a bind mechanism
// this build supports on the configured transport. It has no side effects and
// performs no I/O, so it is safe to call on every path that can reach a bind.
func validateBindConfiguration(config *v3.ActiveDirectoryConfig) error {
	switch config.BindMechanism {
	case "", bindMechanismSimple:
		return nil
	case bindMechanismNTLM:
		if !config.TLS && !config.StartTLS {
			return fmt.Errorf("bindMechanism %q requires tls or starttls", bindMechanismNTLM)
		}
		return nil
	case bindMechanismKerberos:
		return fmt.Errorf("bindMechanism %q is reserved and not yet supported", bindMechanismKerberos)
	default:
		return fmt.Errorf("invalid bindMechanism %q, must be one of %q or %q",
			config.BindMechanism, bindMechanismSimple, bindMechanismNTLM)
	}
}

// errBindConfiguration marks a failure caused by an invalid bind configuration
// rather than by an unreachable or malformed auth config. Callers of
// getActiveDirectoryConfig deliberately swallow generic load failures; this
// sentinel lets them surface the one class of error an operator can act on
// without changing behavior for any other.
var errBindConfiguration = errors.New("invalid Active Directory bind configuration")

// splitNTLMIdentity returns the NetBIOS domain and sAMAccountName the NTLM
// mechanism accepts. It never applies to simple binds, which keep passing the
// raw value through ldap.GetUserExternalID.
//
// Only DOMAIN\user and a bare user name with a configured default login domain
// are accepted. A user principal name needs a search against
// userPrincipalName and a canonicalization step that does not exist yet, so it
// is rejected outright rather than silently treated as a bare name and failing
// later as an unrelated lookup error.
func splitNTLMIdentity(username, defaultDomain string) (string, string, error) {
	invalid := func(reason string) error {
		return apierror.NewAPIError(validation.InvalidOption,
			fmt.Sprintf("username %q cannot be used with bindMechanism %q: %s", username, bindMechanismNTLM, reason))
	}

	if strings.TrimSpace(username) == "" {
		return "", "", invalid("it is empty")
	}
	if strings.Contains(username, "@") {
		return "", "", invalid("user principal names are not supported, use DOMAIN\\user or configure a default login domain")
	}
	if _, err := ldapv3.ParseDN(username); err == nil {
		return "", "", invalid("distinguished names are not supported, use DOMAIN\\user")
	}

	domain := defaultDomain
	user := username

	if parts := strings.Split(username, `\`); len(parts) > 1 {
		if len(parts) > 2 {
			return "", "", invalid("it contains more than one backslash")
		}
		domain, user = parts[0], parts[1]
	}

	if strings.TrimSpace(domain) == "" {
		return "", "", invalid("no domain was given and defaultLoginDomain is not set")
	}
	if strings.TrimSpace(user) == "" {
		return "", "", invalid("the user part is empty")
	}

	return domain, user, nil
}
