package filter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rancher/auth/authenticator"
	"github.com/rancher/auth/util"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func NewAuthenticationFilter(ctx context.Context, managementContext *config.ManagementContext, next http.Handler) (http.Handler, error) {
	if managementContext == nil {
		return nil, fmt.Errorf("Failed to build NewAuthenticationFilter, nil ManagementContext")
	}
	auth := authenticator.NewAuthenticator(ctx, managementContext)
	return &authHeaderHandler{
		auth: auth,
		next: next,
	}, nil
}

type authHeaderHandler struct {
	auth authenticator.Authenticator
	next http.Handler
}

func (h authHeaderHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	authed, user, groups, err := h.auth.Authenticate(req)
	if err != nil || !authed {
		util.ReturnHTTPError(rw, req, 401, err.Error())
		return
	}

	logrus.Debugf("Impersonating user %v, groups %v", user, groups)

	req.Header.Set("Impersonate-User", user)

	req.Header.Del("Impersonate-Group")
	for _, group := range groups {
		req.Header.Add("Impersonate-Group", group)
	}

	h.next.ServeHTTP(rw, req)
}
