package audit

import (
	"context"
	"encoding/json"
	"net/http"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/sirupsen/logrus"
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

	// RequestUser is the --as user
	RequestUser string `json:"requestUser,omitempty"`

	// RequestGroups is the --as-group list
	RequestGroups []string `json:"requestGroups,omitempty"`
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

func getUserInfo(req *http.Request) *User {
	user, _ := request.UserFrom(req.Context())
	extra := user.GetExtra()
	return &User{
		Name:     user.GetName(),
		Username: UsernameFromExtra(extra),
		Group:    user.GetGroups(),
		Extra:    extra,
	}
}

// FromContext gets the user information from the given context.
func FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKeyValue).(*User)
	return u, ok
}
