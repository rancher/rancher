package publicapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/apis/management.cattle.io/v3public/schema"
	"github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

const (
	CookieName = "R_SESS"
)

func newLoginHandler(mgmt *config.ManagementContext) *loginHandler {
	return &loginHandler{
		mgmt: mgmt,
		mgr:  common.NewUserManager(mgmt),
	}
}

type loginHandler struct {
	mgmt *config.ManagementContext
	mgr  common.UserManager
}

func (h *loginHandler) login(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName != "login" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	w := request.Response

	token, responseType, status, err := h.createLoginToken(request)
	if err != nil {
		logrus.Errorf("Login failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
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
			return err
		}
		tokenData["token"] = token.ObjectMeta.Name + ":" + token.Token
		request.WriteResponse(http.StatusCreated, tokenData)
	}

	return nil
}

func (h *loginHandler) createLoginToken(request *types.APIContext) (v3.Token, string, int, error) {
	logrus.Debugf("Create Token Invoked")

	bytes, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		logrus.Errorf("login failed with error: %v", err)
		return v3.Token{}, "", httperror.InvalidBodyContent.Status, httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	generic := &v3public.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", httperror.InvalidBodyContent.Status, httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}
	responseType := generic.ResponseType
	description := generic.Description
	ttl := generic.TTLMillis

	var input interface{}
	var providerName string
	switch request.Type {
	case client.GithubProviderType:
		gInput := &v3public.GithubLogin{}
		input = gInput
		providerName = "github"
	case client.LocalProviderType:
		lInput := &v3public.LocalLogin{}
		input = lInput
		providerName = "local"
	default:
		return v3.Token{}, "", httperror.ServerError.Status, httperror.NewAPIError(httperror.ServerError, "Unknown login type")
	}

	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", httperror.InvalidBodyContent.Status, httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	// Authenticate User
	userPrincipal, groupPrincipals, providerInfo, status, err := providers.AuthenticateUser(input, providerName)
	if status != 0 || err != nil {
		return v3.Token{}, "", status, err
	}

	logrus.Debug("User Authenticated")

	user, err := h.mgr.EnsureUser(userPrincipal.Name, userPrincipal.DisplayName)
	if err != nil {
		return v3.Token{}, "", 500, err
	}

	k8sToken := tokens.GenerateNewLoginToken(user.Name, userPrincipal, groupPrincipals, providerInfo, ttl, description)
	rToken, err := tokens.CreateTokenCR(&k8sToken)
	return rToken, responseType, 0, err
}
