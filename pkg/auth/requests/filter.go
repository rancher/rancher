package requests

import (
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/auth/audit"
	authcontext "github.com/rancher/rancher/pkg/auth/context"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/util"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/apis/authentication"
)

func NewAuthenticatedFilter(next http.Handler) http.Handler {
	return &authHeaderHandler{
		next: next,
	}
}

type authHeaderHandler struct {
	next http.Handler
}

func (h authHeaderHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	userInfo, authed := request.UserFrom(req.Context())
	// checking for system:cattle:error user keeps the old behavior of always returning 401 when authentication fails
	if !authed || userInfo.GetName() == "system:cattle:error" {
		util.ReturnHTTPError(rw, req, 401, ErrMustAuthenticate.Error())
		return
	}

	// clean extra that is not part of userInfo
	for header := range req.Header {
		if key, ok := strings.CutPrefix(header, authentication.ImpersonateUserExtraHeaderPrefix); ok {
			if !providers.IsValidUserExtraAttribute(key) {
				req.Header.Del(header)
			}
		}
	}

	if !authcontext.IsSAAuthenticated(req.Context()) {
		// If the request is not authenticated as a service account,
		// we need to set impersonation headers to the authenticated user
		// impersonation details.
		// This allows the SubjectAccessReviewHandler to work based on the
		// authenticated user instead of the service account.
		req.Header.Set(authentication.ImpersonateUserHeader, userInfo.GetName())
		req.Header.Del(authentication.ImpersonateGroupHeader)
		for _, group := range userInfo.GetGroups() {
			req.Header.Add(authentication.ImpersonateGroupHeader, group)
		}

		// This copies over the extra fields of the user info as impersonation
		// headers, so they can be used by the SubjectAccessReviewHandler if
		// needed.
		for key, extras := range userInfo.GetExtra() {
			for _, s := range extras {
				if s != "" {
					req.Header.Add(authentication.ImpersonateUserExtraHeaderPrefix+key, s)
				}
			}
		}
	}

	auditUser, ok := audit.FromContext(req.Context())
	if ok {
		auditUser.Name = userInfo.GetName()
		auditUser.Group = userInfo.GetGroups()
	}

	h.next.ServeHTTP(rw, req)
}

func NewRequireAuthenticatedFilter(pathPrefix string, ignorePrefix ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &authedFilter{
			next:         next,
			pathPrefix:   pathPrefix,
			ignorePrefix: ignorePrefix,
		}
	}
}

type authedFilter struct {
	next         http.Handler
	pathPrefix   string
	ignorePrefix []string
}

func (h authedFilter) matches(path string) bool {
	if strings.HasPrefix(path, h.pathPrefix) {
		for _, prefix := range h.ignorePrefix {
			if strings.HasPrefix(path, prefix) {
				return false
			}
		}
		return true
	}
	return false
}

func (h authedFilter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h.matches(req.URL.Path) {
		userInfo, authed := request.UserFrom(req.Context())
		// checking for system:cattle:error user keeps the old behavior of always returning 401 when authentication fails
		if !authed || userInfo.GetName() == "system:cattle:error" {
			util.ReturnHTTPError(rw, req, 401, ErrMustAuthenticate.Error())
			return
		}
	}

	h.next.ServeHTTP(rw, req)
}
