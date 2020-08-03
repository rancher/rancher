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
	if !authed {
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
