package audit

import (
	"context"
	"encoding/json"
	"net/http"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// User holds information about the user who caused the audit log
type User struct {
	// Name is the Rancher user ID (for example "user-h9q7h") for authenticated users, or a
	// system identity such as "system:unauthenticated". It is not a login name and is opaque
	// to humans; use Identity to identify a person.
	Name string `json:"name,omitempty"`

	// Identity says who the user is in terms of a person and the auth provider that knows them. It
	// is best-effort and may be omitted. See [Identity].
	Identity *Identity `json:"identity,omitempty"`

	Group []string            `json:"group,omitempty"`
	Extra map[string][]string `json:"extra,omitempty"`

	// ImpersonatedBy is the real authenticated caller. It is set only when the caller successfully
	// impersonated another user, in which case Name, Identity, Group and Extra describe the
	// impersonated (effective) user. A denied impersonation leaves it unset.
	ImpersonatedBy *Impersonator `json:"impersonatedBy,omitempty"`

	// RequestUser is the --as user
	RequestUser string `json:"requestUser,omitempty"`

	// RequestGroups is the --as-group list
	RequestGroups []string `json:"requestGroups,omitempty"`

	// withIdentity is whether Identity and ImpersonatedBy are collected for this entry. It is decided
	// once when the entry is created, from the audit-log-username feature, so when it is off no
	// identity is looked up or assigned.
	withIdentity bool
}

// Identity describes a user as a person known to an auth provider. The user ID in [User].Name stays
// the key for correlation: a username is not unique across providers and can change, so it is
// meaningful together with Provider.
type Identity struct {
	// Username is the human-readable name of the user, such as the login name.
	Username string `json:"username,omitempty"`

	// Provider is the ID of the auth provider that authenticated the session, such as "local" or "github".
	Provider string `json:"provider,omitempty"`

	// PrincipalID is the ID of the provider's principal for the user, such as "github_user://12345".
	PrincipalID string `json:"principalId,omitempty"`
}

// IsZero returns whether the identity holds no information.
func (i Identity) IsZero() bool {
	return i == Identity{}
}

// ref returns a pointer to the identity, or nil when it is empty so that it is omitted from the entry.
func (i Identity) ref() *Identity {
	if i.IsZero() {
		return nil
	}
	return &i
}

// Impersonator identifies the real authenticated caller of an impersonated request.
type Impersonator struct {
	// Name is the Rancher user ID of the caller, with the same meaning as [User].Name.
	Name string `json:"name,omitempty"`

	// Identity is the identity of the caller, with the same meaning as [User].Identity.
	Identity *Identity `json:"identity,omitempty"`

	Group []string `json:"group,omitempty"`
}

// SetIdentity sets Name, Identity and Group to the effective identity of the request from info.
// Group is replaced, not modified in place, because it shares memory with info.
//
// Extra and ImpersonatedBy are left unchanged. Extra is captured when the entry is created, and
// only the SCIM authenticator replaces it, so the content of existing entries does not change.
func (u *User) SetIdentity(info user.Info) {
	u.Name = info.GetName()
	if u.withIdentity {
		u.Identity = IdentityFromInfo(info).ref()
	}
	u.Group = info.GetGroups()
}

// SetImpersonatedBy records real as the authenticated caller that impersonated the effective user. It
// does nothing unless identities are collected for the entry.
func (u *User) SetImpersonatedBy(real user.Info) {
	if !u.withIdentity {
		return
	}
	u.ImpersonatedBy = &Impersonator{
		Name:     real.GetName(),
		Identity: IdentityFromInfo(real).ref(),
		Group:    real.GetGroups(),
	}
}

// Impersonate records that real is impersonating effective: effective becomes the identity of the
// request and real is kept as ImpersonatedBy. Call it where the request user is replaced.
func (u *User) Impersonate(real, effective user.Info) {
	u.SetImpersonatedBy(real)
	u.SetIdentity(effective)
}

// clearGroups removes the groups of the user and of the impersonator, for the ExcludeGroups option.
func (u *User) clearGroups() {
	u.Group = nil
	if u.ImpersonatedBy != nil {
		u.ImpersonatedBy.Group = nil
	}
}

func getUserNameForBasicLogin(body []byte) string {
	input := &v32.BasicLogin{}

	err := json.Unmarshal(body, input)
	if err != nil {
		logrus.Debugf("error unmarshalling user, cannot add login info to audit logEntry: %v", err)
		return ""
	}

	return input.Username
}

// UsernameFromExtra returns the human-readable name from the "username" user extra, or "" if there is none.
// The extra can hold several values, one per linked auth provider, in no meaningful order. The
// smallest non-empty value is returned so the result is stable across requests.
func UsernameFromExtra(extra map[string][]string) string {
	var username string
	for _, name := range extra[common.UserAttributeUserName] {
		if name != "" && (username == "" || name < username) {
			username = name
		}
	}
	return username
}

// fillIdentity sets the username of an empty Identity, and of an empty ImpersonatedBy.Identity, from
// the user record that the respective Name refers to: the user's username, otherwise its display
// name. It runs when the entry is written, so the identities are final. Users without a record, such
// as system identities, keep an empty Identity.
//
// An identity that already has a provider or principal is left alone. The record does not say which
// provider a name came from, so adding it could attribute a name to the wrong provider.
func (lh *LoggingHandler) fillIdentity(u *User) {
	if !u.withIdentity || lh.resolveUser == nil {
		return
	}
	if u.Identity == nil {
		u.Identity = lh.lookupIdentity(u.Name)
	}
	if u.ImpersonatedBy != nil && u.ImpersonatedBy.Identity == nil {
		u.ImpersonatedBy.Identity = lh.lookupIdentity(u.ImpersonatedBy.Name)
	}
}

func (lh *LoggingHandler) lookupIdentity(userID string) *Identity {
	if userID == "" {
		return nil
	}
	username, displayName, ok := lh.resolveUser(userID)
	if !ok {
		return nil
	}
	if username == "" {
		username = displayName
	}
	return Identity{Username: username}.ref()
}

// IdentityInfo is implemented by a user.Info that carries an authoritative identity, chosen by the
// authenticator for the session. See [WithIdentity].
type IdentityInfo interface {
	AuditIdentity() Identity
}

type identityInfo struct {
	user.Info
	identity Identity
}

func (i identityInfo) AuditIdentity() Identity { return i.identity }

// WithIdentity returns info carrying identity as its authoritative identity. It is kept out of the
// user extras on purpose: extras are forwarded as impersonation headers and checked against the
// impersonator role. The identity is lost whenever the request user is replaced, for example by
// impersonation, so it can never describe a different user than the one it was set for. An empty
// identity returns info unchanged.
func WithIdentity(info user.Info, identity Identity) user.Info {
	if info == nil || identity.IsZero() {
		return info
	}
	return identityInfo{Info: info, identity: identity}
}

// IdentityFromInfo returns the identity for info, which is empty if there is none. The authoritative
// identity from [WithIdentity] wins as a whole, because mixing it with another source could pair a
// username with the wrong provider. Otherwise the identity has only the username from the user extras.
func IdentityFromInfo(info user.Info) Identity {
	if i, ok := info.(IdentityInfo); ok {
		if identity := i.AuditIdentity(); !identity.IsZero() {
			return identity
		}
	}
	return Identity{Username: UsernameFromExtra(info.GetExtra())}
}

func getUserInfo(req *http.Request, withIdentity bool) *User {
	info, _ := request.UserFrom(req.Context())
	u := &User{Extra: info.GetExtra(), withIdentity: withIdentity}
	u.SetIdentity(info)
	return u
}

// FromContext gets the user information from the given context.
func FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKeyValue).(*User)
	return u, ok
}
