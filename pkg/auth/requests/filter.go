package requests

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/audit"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	k8sUser "k8s.io/apiserver/pkg/authentication/user"
)

func NewAuthenticationFilter(ctx context.Context, auth Authenticator, managementContext *config.ScaledContext, next http.Handler, sar sar.SubjectAccessReview) (http.Handler, error) {
	if managementContext == nil {
		return nil, fmt.Errorf("Failed to build NewAuthenticationFilter, nil ManagementContext")
	}
	return &authHeaderHandler{
		auth:              auth,
		next:              next,
		sar:               sar,
		userAuthRefresher: providerrefresh.NewUserAuthRefresher(ctx, managementContext),
	}, nil
}

type authHeaderHandler struct {
	auth              Authenticator
	next              http.Handler
	sar               sar.SubjectAccessReview
	userAuthRefresher providerrefresh.UserAuthRefresher
}

func (h authHeaderHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	authed, user, groups, err := h.auth.Authenticate(req)
	if err != nil || !authed {
		util.ReturnHTTPError(rw, req, 401, err.Error())
		return
	}

	var impersonateUser bool
	var impersonateGroup bool

	reqUser := req.Header.Get("Impersonate-User")
	var reqGroup []string
	if g, ok := req.Header["Impersonate-Group"]; ok {
		reqGroup = g
	}

	// If there is an impersonate header, the incoming request is attempting to
	// impersonate a different user, verify the token user is authz to impersonate
	if h.sar != nil {
		if reqUser != "" && reqUser != user {
			canDo, err := h.sar.UserCanImpersonateUser(req, user, reqUser)
			if err != nil {
				util.ReturnHTTPError(rw, req, 401, err.Error())
				return
			} else if !canDo {
				util.ReturnHTTPError(rw, req, 401, "not allowed to impersonate")
				return
			}
			impersonateUser = true
		}

		if len(reqGroup) > 0 && !groupsEqual(reqGroup, groups) {
			canDo, err := h.sar.UserCanImpersonateGroups(req, user, reqGroup)
			if err != nil {
				util.ReturnHTTPError(rw, req, 401, err.Error())
				return
			} else if !canDo {
				util.ReturnHTTPError(rw, req, 401, "not allowed to impersonate")
				return
			}
			impersonateGroup = true
		}
	}

	// Not impersonating either, set to the token users creds - this is the default
	// case if sar is nil
	if !impersonateUser && !impersonateGroup {
		req.Header.Set("Impersonate-User", user)
		req.Header.Del("Impersonate-Group")
		for _, group := range groups {
			req.Header.Add("Impersonate-Group", group)
		}
	} else if impersonateUser && !impersonateGroup { // Impersonating user, only user the authd header
		req.Header.Set("Impersonate-Group", k8sUser.AllAuthenticated)
	} else if !impersonateUser && impersonateGroup { // Impersonating groups, drop the user, add authd header
		req.Header.Del("Impersonate-User")
		req.Header.Add("Impersonate-Group", k8sUser.AllAuthenticated)
	}

	logrus.Debugf("Requesting user: %v, Requesting groups: %v Impersonate user: %v Impersonate group: %v",
		user, groups, reqUser, reqGroup)

	auditUser, ok := audit.FromContext(req.Context())
	if ok {
		auditUser.Name = user
		auditUser.Group = groups
		auditUser.RequestUser = reqUser
		auditUser.RequestGroups = reqGroup
	}

	if !strings.HasPrefix(user, "system:") {
		go h.userAuthRefresher.TriggerUserRefresh(user, false)
	}

	h.next.ServeHTTP(rw, req)
}

func groupsEqual(group1, group2 []string) bool {
	if len(group1) != len(group2) {
		return false
	}

	sort.Strings(group1)
	sort.Strings(group2)
	return reflect.DeepEqual(group1, group2)
}
