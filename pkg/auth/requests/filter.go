package requests

import (
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/util"
)

func NewAuthenticationFilter(auth Authenticator, next http.Handler) (http.Handler, error) {
	return &authHeaderHandler{
		auth: auth,
		next: next,
	}, nil
}

type authHeaderHandler struct {
	auth Authenticator
	next http.Handler
}

func (h authHeaderHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	authed, user, groups, err := h.auth.Authenticate(req)
	if err != nil || !authed {
		util.ReturnHTTPError(rw, req, 401, err.Error())
		return
	}

	// clean extra
	for header := range req.Header {
		if strings.HasPrefix(header, "Impersonate-Extra-") {
			req.Header.Del(header)
		}
	}

	req.Header.Set("Impersonate-User", user)
	req.Header.Del("Impersonate-Group")
	for _, group := range groups {
		req.Header.Add("Impersonate-Group", group)
	}

	auditUser, ok := audit.FromContext(req.Context())
	if ok {
		auditUser.Name = user
		auditUser.Group = groups
	}

	h.next.ServeHTTP(rw, req)
}
