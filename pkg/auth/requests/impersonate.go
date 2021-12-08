package requests

import (
	"errors"
	"net/http"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/steve/pkg/auth"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sUser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type impersonatingAuth struct {
	sar sar.SubjectAccessReview
}

func NewImpersonatingAuth(sar sar.SubjectAccessReview) auth.Authenticator {
	return &impersonatingAuth{
		sar: sar,
	}
}

func (h *impersonatingAuth) Authenticate(req *http.Request) (k8sUser.Info, bool, error) {
	userInfo, authed := request.UserFrom(req.Context())
	if !authed {
		return nil, false, nil
	}
	user := userInfo.GetName()
	groups := userInfo.GetGroups()

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
				return nil, false, err
			} else if !canDo {
				return nil, false, errors.New("not allowed to impersonate")
			}
			impersonateUser = true
		}

		if len(reqGroup) > 0 && !groupsEqual(reqGroup, groups) {
			canDo, err := h.sar.UserCanImpersonateGroups(req, user, reqGroup)
			if err != nil {
				return nil, false, err
			} else if !canDo {
				return nil, false, errors.New("not allowed to impersonate")
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

	extra := userInfo.GetExtra()

	return &k8sUser.DefaultInfo{
		Name:   user,
		UID:    user,
		Groups: groups,
		Extra:  extra,
	}, true, nil
}

func groupsEqual(group1, group2 []string) bool {
	if len(group1) != len(group2) {
		return false
	}

	return sets.NewString(group1...).Equal(sets.NewString(group2...))
}
