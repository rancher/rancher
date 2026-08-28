package activedirectory

import (
	"errors"
	"fmt"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/apiserver/pkg/apierror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap/ntlm"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

// Values of ActiveDirectoryConfig.BindMechanism.
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

// ntlmIdentity is a bind identity already resolved by splitNTLMIdentity.
type ntlmIdentity struct {
	Domain   string
	Username string
}

// ntlmChallengeBinder is the part of *ldapv3.Conn that performs an NTLM bind.
// ldapv3.Client does not include NTLMChallengeBind, so the concrete capability
// is asserted at the call site.
type ntlmChallengeBinder interface {
	NTLMChallengeBind(request *ldapv3.NTLMBindRequest) (*ldapv3.NTLMBindResult, error)
}

// bindServiceAccount binds conn as the configured service account.
//
// The bind error is returned unclassified. Sites that build an API response
// pass it through classifyServiceAccountBindError; sites with a config.Enabled
// fallback must not, because that fallback inspects the LDAP result code.
func (p *adProvider) bindServiceAccount(conn ldapv3.Client, config *v3.ActiveDirectoryConfig) error {
	logrus.Debug("Binding service account username password")

	if config.ServiceAccountPassword == "" {
		return apierror.NewAPIError(validation.MissingRequired, "service account password not provided")
	}
	return p.bindAs(conn, config, nil, config.ServiceAccountUsername, config.ServiceAccountPassword)
}

// bindUser binds conn as an end user. identity is optional: when nil, the
// NTLM path in bindAs parses username itself.
func (p *adProvider) bindUser(conn ldapv3.Client, config *v3.ActiveDirectoryConfig, identity *ntlmIdentity, username, password string) error {
	return p.bindAs(conn, config, identity, username, password)
}

// bindAs binds conn according to config.BindMechanism. It is the only place in
// the provider that performs a bind, so no path can reach the directory with a
// mechanism the configuration did not select.
//
// identity, when non-nil, is an already-resolved NTLM identity and takes
// precedence over parsing username.
//
// Failures bindAs detects itself — an invalid mechanism, a connection
// without TLS state, a missing peer certificate, an underivable channel
// binding token — are returned as APIErrors carrying actionable messages.
// Only an error from the directory comes back unclassified.
func (p *adProvider) bindAs(conn ldapv3.Client, config *v3.ActiveDirectoryConfig, identity *ntlmIdentity, username, password string) error {
	// Defense in depth: the config is validated where it is loaded and where
	// it is applied, but a future caller could reach here another way.
	if err := validateBindConfiguration(config); err != nil {
		return apierror.WrapAPIError(err, validation.InvalidOption, err.Error())
	}

	if config.BindMechanism != bindMechanismNTLM {
		return conn.Bind(ldap.GetUserExternalID(username, config.DefaultLoginDomain), password)
	}

	binder, ok := conn.(ntlmChallengeBinder)
	if !ok {
		// A programming error, not a configuration one. Never fall back to a
		// simple bind: that would silently defeat channel binding.
		return apierror.NewAPIError(validation.ServerError,
			fmt.Sprintf("activedirectory: connection of type %T cannot perform an NTLM bind", conn))
	}

	if identity == nil {
		domain, user, err := splitNTLMIdentity(username, config.DefaultLoginDomain)
		if err != nil {
			return err
		}
		identity = &ntlmIdentity{Domain: domain, Username: user}
	}

	state, ok := conn.TLSConnectionState()
	if !ok {
		return apierror.NewAPIError(validation.ServerError,
			"activedirectory: an NTLM bind needs a TLS connection, but the connection is not using TLS")
	}
	if len(state.PeerCertificates) == 0 {
		return apierror.NewAPIError(validation.ServerError,
			"activedirectory: an NTLM bind needs the server certificate to derive a channel binding token, but the TLS connection presented none")
	}

	cbt, err := ntlm.ChannelBindingToken(state.PeerCertificates[0])
	if err != nil {
		return apierror.WrapAPIError(err, validation.ServerError, err.Error())
	}

	negotiator, err := ntlm.NewNegotiator(cbt)
	if err != nil {
		return apierror.WrapAPIError(err, validation.ServerError, err.Error())
	}

	logrus.Debugf("activedirectory: binding %s with mechanism %s", identity.Domain, bindMechanismNTLM)

	// Password only. Leaving Hash empty keeps pass-the-hash unreachable and
	// makes go-ldap derive the NT hash it passes to the negotiator.
	//
	// The error is returned unwrapped. Callers inspect it with
	// ldapv3.IsErrorWithCode, and classification into an APIError happens at
	// the call sites that produce an API response — never here.
	_, err = binder.NTLMChallengeBind(&ldapv3.NTLMBindRequest{
		Domain:     identity.Domain,
		Username:   identity.Username,
		Password:   password,
		Negotiator: negotiator,
	})
	return err
}

// classifyServiceAccountBindError turns a service-account bind failure into
// the APIError the login and refresh paths have always returned.
//
// It replaces the classification that lived in ldap.AuthenticateServiceAccountUser.
// The channel-binding case is separated out because it arrives as
// invalidCredentials but is a configuration fault, not a wrong password, and
// apierror.WrapAPIError drops the cause from the response — so the mapped
// explanation has to be the message.
func classifyServiceAccountBindError(err error) error {
	if err == nil {
		return nil
	}

	// bindServiceAccount and bindAs already return APIErrors for the problems
	// they detect themselves (see the bindAs doc for the list). Those carry
	// codes and messages an operator can act on, so they pass through
	// untouched. Only an error from the directory reaches the LDAP
	// classification below.
	if apierror.IsAPIError(err) {
		return err
	}

	// Both non-credential failures arrive as result code 49 and must be
	// separated from a wrong password before the Unauthorized branch below.
	if classifyBindFailure(err) != bindFailureNone {
		return apierror.WrapAPIError(err, validation.ServerError, mapBindError(err).Error())
	}
	if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultInvalidCredentials) {
		return apierror.WrapAPIError(err, validation.Unauthorized, "authentication failed")
	}
	return apierror.WrapAPIError(err, validation.ServerError, "server error while authenticating")
}
