package requests

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/audit"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/types/config"
)

func NewAuthenticationFilter(ctx context.Context, auth Authenticator, managementContext *config.ScaledContext, next http.Handler) (http.Handler, error) {
	if managementContext == nil {
		return nil, fmt.Errorf("Failed to build NewAuthenticationFilter, nil ManagementContext")
	}
	return &authHeaderHandler{
		auth:              auth,
		next:              next,
		userAuthRefresher: providerrefresh.NewUserAuthRefresher(ctx, managementContext),
	}, nil
}

type authHeaderHandler struct {
	auth              Authenticator
	next              http.Handler
	userAuthRefresher providerrefresh.UserAuthRefresher
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

	if !strings.HasPrefix(user, "system:") {
		go h.userAuthRefresher.TriggerUserRefresh(user, false)
	}

	h.next.ServeHTTP(rw, req)
}
