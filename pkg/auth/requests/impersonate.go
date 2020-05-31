package requests

import (
	"errors"
	"net/http"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sUser "k8s.io/apiserver/pkg/authentication/user"
)

type impersonatingAuth struct {
	Authenticator
	sar sar.SubjectAccessReview
}

func NewImpersonatingAuth(next Authenticator, sar sar.SubjectAccessReview) Authenticator {
	return &impersonatingAuth{
		Authenticator: next,
		sar:           sar,
	}
}

func (h *impersonatingAuth) Authenticate(req *http.Request) (authed bool, user string, groups []string, err error) {
	authed, user, groups, err = h.Authenticator.Authenticate(req)
	if err != nil || !authed {
		return authed, user, groups, err
	}

	var impersonateUser bool
	var impersonateGroup bool

	reqUser := req.Header.Get("Impersonate-User")
	var reqGroup []string
	if g, ok := req.Header["Impersonate-Group"]; ok {
		reqGroup = g
	}

	auditUser, ok := audit.FromContext(req.Context())
	if ok {
		auditUser.RequestUser = reqUser
		auditUser.RequestGroups = reqGroup
	}

	// If there is an impersonate header, the incoming request is attempting to
	// impersonate a different user, verify the token user is authz to impersonate
	if h.sar != nil {
		if reqUser != "" && reqUser != user {
			canDo, err := h.sar.UserCanImpersonateUser(req, user, reqUser)
			if err != nil {
				return false, user, groups, err
			} else if !canDo {
				return false, user, groups, errors.New("not allowed to impersonate")
			}
			impersonateUser = true
		}

		if len(reqGroup) > 0 && !groupsEqual(reqGroup, groups) {
			canDo, err := h.sar.UserCanImpersonateGroups(req, user, reqGroup)
			if err != nil {
				return false, user, groups, err
			} else if !canDo {
				return false, user, groups, errors.New("not allowed to impersonate")
			}
			impersonateGroup = true
		}
	}

	if impersonateUser || impersonateGroup {
		if impersonateUser {
			user = reqUser
		}
		if impersonateGroup {
			groups = reqGroup
		} else {
			groups = nil
		}
		groups = append(groups, k8sUser.AllAuthenticated)
	}

	return true, user, groups, nil
}

func groupsEqual(group1, group2 []string) bool {
	if len(group1) != len(group2) {
		return false
	}

	return sets.NewString(group1...).Equal(sets.NewString(group2...))
}
