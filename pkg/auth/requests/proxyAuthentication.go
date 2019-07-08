package requests

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	k8sHeaderRequest "k8s.io/apiserver/pkg/authentication/request/headerrequest"
)

type ProxyAuthenticator interface {
	Authenticate(req *http.Request) (authed bool, user string, groups []string, err error)
}

func NewProxyAuthenticator(ctx context.Context, mgmtCtx *config.ScaledContext) ProxyAuthenticator {
	return &proxyAuthenticator{
		ctx:     ctx,
		userMGR: mgmtCtx.UserManager,
	}
}

type proxyAuthenticator struct {
	ctx     context.Context
	userMGR user.Manager
}

const (
	proxyCAFile = "/etc/rancher/ssl/proxy-authentication-ca.pem" // PEM-encoded certificate bundle
)

func (a *proxyAuthenticator) Authenticate(req *http.Request) (bool, string, []string, error) {
	remoteUserName, remoteUserGroups, err := validateRequestCA(req)
	if err != nil {
		return false, "", []string{}, err
	}
	// Remote header can be in format u-123 or local://u-123
	// But we have to use local:// version past this point for lookups
	if !strings.Contains(remoteUserName, "local://") {
		remoteUserName = "local://" + remoteUserName
	}
	remoteUser, err := a.userMGR.GetUserByPrincipalID(remoteUserName)
	if err != nil {
		return false, "", []string{}, err
	}
	if remoteUser == nil {
		return false, "", []string{}, fmt.Errorf("user '%v' not found", remoteUserName)
	}
	if remoteUser.Enabled != nil && !*remoteUser.Enabled {
		return false, "", []string{}, fmt.Errorf("user '%v' not enabled", remoteUserName)
	}
	return true, remoteUser.ObjectMeta.Name, remoteUserGroups, nil
}

func validateRequestCA(req *http.Request) (string, []string, error) {
	auth, err := k8sHeaderRequest.NewSecure(
		proxyCAFile,
		[]string{}, // Not whitelisting common names
		[]string{"X-Remote-User"},
		[]string{"X-Remote-Group"},
		[]string{"X-Remote-Extra-"},
	)
	if err != nil {
		logrus.Errorf("Failed to validate proxy authenticator CA: %v", err.Error())
		return "", []string{}, fmt.Errorf("failed to validate proxy authenticator CA")
	}
	authenticatorResponse, mTLSAuthed, err := auth.AuthenticateRequest(req)
	if err != nil {
		return "", []string{}, err
	}
	if mTLSAuthed != true {
		return "", []string{}, fmt.Errorf("mTLS Invalid")
	}
	return authenticatorResponse.GetName(), authenticatorResponse.GetGroups(), nil
}
