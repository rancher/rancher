package activedirectory

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
