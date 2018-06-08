package publicapi

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/openldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/apis/management.cattle.io/v3public/schema"
	"github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
)

const (
	CookieName = "R_SESS"
)

func newLoginHandler(mgmt *config.ScaledContext) *loginHandler {
	return &loginHandler{
		mgr: mgmt.UserManager,
	}
}

type loginHandler struct {
	mgr user.Manager
}

func (h *loginHandler) login(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName != "login" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	w := request.Response

	token, responseType, err := h.createLoginToken(request)
	if err != nil {
		// if user fails to authenticate, hide the details of the exact error. bad credentials will already be APIErrors
		// otherwise, return a generic error message
		if httperror.IsAPIError(err) {
			return err
		}
		return httperror.WrapAPIError(err, httperror.ServerError, "Server error while authenticating")
	}

	if responseType == "cookie" {
		tokenCookie := &http.Cookie{
			Name:     CookieName,
			Value:    token.ObjectMeta.Name + ":" + token.Token,
			Secure:   true,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)
	} else {
		tokenData, err := tokens.ConvertTokenResource(request.Schemas.Schema(&schema.PublicVersion, client.TokenType), token)
		if err != nil {
			return httperror.WrapAPIError(err, httperror.ServerError, "Server error while authenticating")
		}
		tokenData["token"] = token.ObjectMeta.Name + ":" + token.Token
		request.WriteResponse(http.StatusCreated, tokenData)
	}

	return nil
}

func (h *loginHandler) createLoginToken(request *types.APIContext) (v3.Token, string, error) {
	logrus.Debugf("Create Token Invoked")

	bytes, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		logrus.Errorf("login failed with error: %v", err)
		return v3.Token{}, "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	generic := &v3public.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}
	responseType := generic.ResponseType
	description := generic.Description
	ttl := generic.TTLMillis

	var input interface{}
	var providerName string
	switch request.Type {
	case client.LocalProviderType:
		input = &v3public.BasicLogin{}
		providerName = local.Name
	case client.GithubProviderType:
		input = &v3public.GithubLogin{}
		providerName = github.Name
	case client.ActiveDirectoryProviderType:
		input = &v3public.BasicLogin{}
		providerName = activedirectory.Name
	case client.OpenLDAPProviderType:
		input = &v3public.BasicLogin{}
		providerName = openldap.Name
	default:
		return v3.Token{}, "", httperror.NewAPIError(httperror.ServerError, "unknown authentication provider")
	}

	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	// Authenticate User
	userPrincipal, groupPrincipals, providerInfo, err := providers.AuthenticateUser(input, providerName)
	if err != nil {
		return v3.Token{}, "", err
	}

	logrus.Debug("User Authenticated")

	displayName := userPrincipal.DisplayName
	if displayName == "" {
		displayName = userPrincipal.LoginName
	}
	user, err := h.mgr.EnsureUser(userPrincipal.Name, displayName)
	if err != nil {
		return v3.Token{}, "", err
	}

	rToken, err := tokens.NewLoginToken(user.Name, userPrincipal, groupPrincipals, providerInfo, ttl, description)
	return rToken, responseType, err
}
