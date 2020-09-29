package requests

import (
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/util"
	"k8s.io/apiserver/pkg/endpoints/request"
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

	// clean extra
	for header := range req.Header {
		if strings.HasPrefix(header, "Impersonate-Extra-") {
			req.Header.Del(header)
		}
	}

	req.Header.Set("Impersonate-User", userInfo.GetName())
	req.Header.Del("Impersonate-Group")
	for _, group := range userInfo.GetGroups() {
		req.Header.Add("Impersonate-Group", group)
	}

	auditUser, ok := audit.FromContext(req.Context())
	if ok {
		auditUser.Name = userInfo.GetName()
		auditUser.Group = userInfo.GetGroups()
	}

	h.next.ServeHTTP(rw, req)
}

func NewRequireAuthenticatedFilter(pathPrefix string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &authedFilter{
			next:       next,
			pathPrefix: pathPrefix,
		}
	}
}

type authedFilter struct {
	next       http.Handler
	pathPrefix string
}

func (h authedFilter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, h.pathPrefix) {
		userInfo, authed := request.UserFrom(req.Context())
		// checking for system:cattle:error user keeps the old behavior of always returning 401 when authentication fails
		if !authed || userInfo.GetName() == "system:cattle:error" {
			util.ReturnHTTPError(rw, req, 401, ErrMustAuthenticate.Error())
			return
		}
	}

	h.next.ServeHTTP(rw, req)
}
