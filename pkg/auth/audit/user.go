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
	// to humans; use Username to identify a person.
	Name string `json:"name,omitempty"`

	// Username is the human-readable name of the user. It is best-effort and may be empty.
	// It is not guaranteed to be unique and can change over time, so use Name for correlation.
	Username string `json:"username,omitempty"`

	Group []string            `json:"group,omitempty"`
	Extra map[string][]string `json:"extra,omitempty"`

	// ImpersonatedBy is the real authenticated caller. It is set only when the caller successfully
	// impersonated another user, in which case Name, Username, Group and Extra describe the
	// impersonated (effective) user. A denied impersonation leaves it unset.
	ImpersonatedBy *Impersonator `json:"impersonatedBy,omitempty"`

	// RequestUser is the --as user
	RequestUser string `json:"requestUser,omitempty"`

	// RequestGroups is the --as-group list
	RequestGroups []string `json:"requestGroups,omitempty"`
}

// Impersonator identifies the real authenticated caller of an impersonated request.
type Impersonator struct {
	// Name is the Rancher user ID of the caller, with the same meaning as [User].Name.
	Name string `json:"name,omitempty"`

	// Username is the human-readable name of the caller, with the same meaning as [User].Username.
	Username string `json:"username,omitempty"`

	Group []string `json:"group,omitempty"`
}

// SetIdentity sets Name, Username and Group to the effective identity of the request from info.
// Group is replaced, not modified in place, because it shares memory with info.
//
// Extra and ImpersonatedBy are left unchanged. Extra is captured when the entry is created, and
// only the SCIM authenticator replaces it, so the content of existing entries does not change.
func (u *User) SetIdentity(info user.Info) {
	u.Name = info.GetName()
	u.Username = UsernameFromInfo(info)
	u.Group = info.GetGroups()
}

// SetImpersonatedBy records real as the authenticated caller that impersonated the effective user.
func (u *User) SetImpersonatedBy(real user.Info) {
	u.ImpersonatedBy = &Impersonator{
		Name:     real.GetName(),
		Username: UsernameFromInfo(real),
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

// fillUsername sets an empty Username, and an empty ImpersonatedBy.Username, from the user record
// that the respective Name refers to: the user's username, otherwise its display name. It runs when
// the entry is written, so the names are final. Users without a record, such as system identities,
// keep an empty Username.
func (lh *LoggingHandler) fillUsername(u *User) {
	if lh.resolveUser == nil {
		return
	}
	if u.Username == "" {
		u.Username = lh.lookupUsername(u.Name)
	}
	if u.ImpersonatedBy != nil && u.ImpersonatedBy.Username == "" {
		u.ImpersonatedBy.Username = lh.lookupUsername(u.ImpersonatedBy.Name)
	}
}

func (lh *LoggingHandler) lookupUsername(userID string) string {
	if userID == "" {
		return ""
	}
	username, displayName, ok := lh.resolveUser(userID)
	if !ok {
		return ""
	}
	if username != "" {
		return username
	}
	return displayName
}

// UsernameInfo is implemented by a user.Info that carries an authoritative human-readable name,
// chosen by the authenticator for the session. See [WithUsername].
type UsernameInfo interface {
	AuditUsername() string
}

type usernameInfo struct {
	user.Info
	username string
}

func (u usernameInfo) AuditUsername() string { return u.username }

// WithUsername returns info carrying username as its authoritative human-readable name. It is kept
// out of the user extras on purpose: extras are forwarded as impersonation headers and checked
// against the impersonator role. The name is lost whenever the request user is replaced, for
// example by impersonation, so it can never describe a different user than the one it was set for.
// An empty username returns info unchanged.
func WithUsername(info user.Info, username string) user.Info {
	if info == nil || username == "" {
		return info
	}
	return usernameInfo{Info: info, username: username}
}

// UsernameFromInfo returns the human-readable name for info, or "" if there is none. The
// authoritative name from [WithUsername] wins, otherwise it is taken from the user extras.
func UsernameFromInfo(info user.Info) string {
	if u, ok := info.(UsernameInfo); ok {
		if username := u.AuditUsername(); username != "" {
			return username
		}
	}
	return UsernameFromExtra(info.GetExtra())
}

func getUserInfo(req *http.Request) *User {
	info, _ := request.UserFrom(req.Context())
	u := &User{Extra: info.GetExtra()}
	u.SetIdentity(info)
	return u
}

// FromContext gets the user information from the given context.
func FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKeyValue).(*User)
	return u, ok
}
