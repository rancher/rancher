package filter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/rancher/rancher/pkg/audit"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/types/config"
)

type wrapWriter struct {
	auditID k8stypes.UID
	http.ResponseWriter
	auditWriter *audit.LogWriter
	statusCode  int
}

func (aw *wrapWriter) WriteHeader(statusCode int) {
	aw.ResponseWriter.WriteHeader(statusCode)
	aw.statusCode = statusCode
}

func (aw *wrapWriter) Write(body []byte) (int, error) {
	n, err := aw.ResponseWriter.Write(body)

	go func() {
		aw.auditWriter.LogResponse(body, aw.auditID, aw.statusCode, aw.Header().Get("Content-Type"))
	}()
	return n, err
}

func (aw *wrapWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := aw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("the ResponseWriter doesn't support the Hijacker interface")
}

func NewAuthenticationFilter(ctx context.Context, managementContext *config.ScaledContext, auditWriter *audit.LogWriter, next http.Handler) (http.Handler, error) {
	if managementContext == nil {
		return nil, fmt.Errorf("Failed to build NewAuthenticationFilter, nil ManagementContext")
	}
	auth := requests.NewAuthenticator(ctx, managementContext)

	return &authHandler{
		auth:        auth,
		next:        next,
		auditWriter: auditWriter,
	}, nil
}

type authHandler struct {
	auth        requests.Authenticator
	next        http.Handler
	auditWriter *audit.LogWriter
}

func (h authHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
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

	aw := rw
	if h.auditWriter != nil {
		auditID := k8stypes.UID(uuid.NewRandom().String())
		h.auditWriter.LogRequest(req, auditID, authed, req.Header.Get("Content-Type"), user, groups)

		aw = &wrapWriter{auditID, rw, h.auditWriter, http.StatusOK}
	}

	h.next.ServeHTTP(aw, req)
}
