package audit

import (
	"context"
	"encoding/json"
	"net/http"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// User holds information about the user who caused the audit log
type User struct {
	Name  string              `json:"name,omitempty"`
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
		logrus.Debugf("error unmarshalling user, cannot add login info to audit log: %v", err)
		return ""
	}

	return input.Username
}

func getUserInfo(req *http.Request) *User {
	user, _ := request.UserFrom(req.Context())
	return &User{
		Name:  user.GetName(),
		Group: user.GetGroups(),
		Extra: user.GetExtra(),
	}
}

// FromContext gets the user information from the given context.
func FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	return u, ok
}
