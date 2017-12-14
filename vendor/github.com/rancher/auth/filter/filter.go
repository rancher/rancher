package filter

import (
	"context"
	"net/http"

	"github.com/rancher/auth/authenticator"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func NewAuthenticationFilter(ctx context.Context, managementContext *config.ManagementContext, next http.Handler) (http.Handler, error) {
	auth := authenticator.NewAuthenticator()
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
	if err != nil {
		logrus.Errorf("Error encountered while authenticating: %v", err)
		// TODO who will handle standardizing the format of 400/500 response bodies?
		http.Error(rw, "The server encountered a problem", 500)
		return
	}

	if !authed {
		http.Error(rw, "Failed authentication", 401)
	}

	logrus.Debugf("Impersonating user %v, groups %v", user, groups)

	req.Header.Set("Impersonate-User", user)

	req.Header.Del("Impersonate-Group")
	for _, group := range groups {
		req.Header.Add("Impersonate-Group", group)
	}

	h.next.ServeHTTP(rw, req)
}
