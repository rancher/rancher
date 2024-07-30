package publicapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/keycloakoidc"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/util"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
)

const (
	CookieName = "R_SESS"
)

func newLoginHandler(ctx context.Context, mgmt *config.ScaledContext) *loginHandler {
	return &loginHandler{
		scaledContext: mgmt,
		userMGR:       mgmt.UserManager,
		tokenMGR:      tokens.NewManager(ctx, mgmt),
		clusterLister: mgmt.Management.Clusters("").Controller().Lister(),
		secretLister:  mgmt.Core.Secrets("").Controller().Lister(),
	}
}

type loginHandler struct {
	scaledContext *config.ScaledContext
	userMGR       user.Manager
	tokenMGR      *tokens.Manager
	clusterLister v3.ClusterLister
	secretLister  v1.SecretLister
}

func (h *loginHandler) login(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName != "login" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	w := request.Response

	token, unhashedTokenKey, responseType, err := h.createLoginToken(request)
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
			Value:    token.ObjectMeta.Name + ":" + unhashedTokenKey,
			Secure:   true,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)
	} else if responseType == "saml" {
		return nil
	} else {
		tokenData, err := tokens.ConvertTokenResource(request.Schemas.Schema(&schema.PublicVersion, client.TokenType), token)
		if err != nil {
			return httperror.WrapAPIError(err, httperror.ServerError, "Server error while authenticating")
		}
		tokenData["token"] = token.ObjectMeta.Name + ":" + unhashedTokenKey
		request.WriteResponse(http.StatusCreated, tokenData)
	}

	return nil
}

// createLoginToken returns token, unhashed token key (where applicable), responseType and error
func (h *loginHandler) createLoginToken(request *types.APIContext) (v3.Token, string, string, error) {
	var userPrincipal v3.Principal
	var groupPrincipals []v3.Principal
	var providerToken string
	logrus.Debugf("Create Token Invoked")

	bytes, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		logrus.Errorf("login failed with error: %v", err)
		return v3.Token{}, "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	generic := &apiv3.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}
	responseType := generic.ResponseType
	description := generic.Description
	ttl := generic.TTLMillis

	authTimeout := settings.AuthUserSessionTTLMinutes.Get()
	if minutes, err := strconv.ParseInt(authTimeout, 10, 64); err == nil {
		ttl = minutes * 60 * 1000
	}

	var input interface{}
	var providerName string
	switch request.Type {
	case client.LocalProviderType:
		input = &apiv3.BasicLogin{}
		providerName = local.Name
	case client.GithubProviderType:
		input = &apiv3.GithubLogin{}
		providerName = github.Name
	case client.ActiveDirectoryProviderType:
		input = &apiv3.BasicLogin{}
		providerName = activedirectory.Name
	case client.AzureADProviderType:
		input = &apiv3.AzureADLogin{}
		providerName = azure.Name
	case client.OpenLdapProviderType:
		input = &apiv3.BasicLogin{}
		providerName = ldap.OpenLdapName
	case client.FreeIpaProviderType:
		input = &apiv3.BasicLogin{}
		providerName = ldap.FreeIpaName
	case client.PingProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.PingName
	case client.ADFSProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.ADFSName
	case client.KeyCloakProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.KeyCloakName
	case client.OKTAProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.OKTAName
	case client.ShibbolethProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.ShibbolethName
	case client.GoogleOAuthProviderType:
		input = &apiv3.GoogleOauthLogin{}
		providerName = googleoauth.Name
	case client.OIDCProviderType:
		input = &apiv3.OIDCLogin{}
		providerName = oidc.Name
	case client.KeyCloakOIDCProviderType:
		input = &apiv3.OIDCLogin{}
		providerName = keycloakoidc.Name
	default:
		return v3.Token{}, "", "", httperror.NewAPIError(httperror.ServerError, "unknown authentication provider")
	}

	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	// Authenticate User
	// SAML's login flow is different from the other providers. Unlike the other providers, it gets the logged in user's data via a POST from
	// the identity provider on a separate endpoint specifically for that.

	if providerName == saml.PingName || providerName == saml.ADFSName || providerName == saml.KeyCloakName ||
		providerName == saml.OKTAName || providerName == saml.ShibbolethName {
		err = saml.PerformSamlLogin(providerName, request, input)
		return v3.Token{}, "", "saml", err
	}

	ctx := context.WithValue(request.Request.Context(), util.RequestKey, request.Request)
	userPrincipal, groupPrincipals, providerToken, err = providers.AuthenticateUser(ctx, input, providerName)
	if err != nil {
		return v3.Token{}, "", "", err
	}

	displayName := userPrincipal.DisplayName
	if displayName == "" {
		displayName = userPrincipal.LoginName
	}

	var backoff = wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Jitter:   .2,
		Steps:    5,
	}

	var (
		enabled  bool
		currUser *v3.User
	)

	err = wait.ExponentialBackoffWithContext(ctx, backoff, func(_ context.Context) (bool, error) {
		var err error

		currUser, err = h.userMGR.EnsureUser(userPrincipal.Name, displayName)
		if err != nil {
			logrus.Warnf("Error creating or updating user for %s, retrying: %v", currUser.Name, err)
			return false, nil
		}

		enabled = pointer.BoolDeref(currUser.Enabled, true)
		if !enabled {
			return true, nil
		}

		loginTime := time.Now()
		userExtraInfo := providers.GetUserExtraAttributes(providerName, userPrincipal)
		err = h.tokenMGR.UserAttributeCreateOrUpdate(currUser.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo, loginTime)
		if err != nil {
			logrus.Warnf("Error creating or updating userAttribute for %s, retrying: %v", currUser.Name, err)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return v3.Token{}, "", "", fmt.Errorf("error creating or updating user and/or userAttribute for %s: %w", currUser.Name, err)
	}

	if !enabled {
		return v3.Token{}, "", "", httperror.NewAPIError(httperror.PermissionDenied, "Permission Denied")
	}

	if strings.HasPrefix(responseType, tokens.KubeconfigResponseType) {
		token, tokenValue, err := tokens.GetKubeConfigToken(currUser.Name, responseType, h.userMGR, userPrincipal)
		if err != nil {
			return v3.Token{}, "", "", err
		}
		return *token, tokenValue, responseType, nil
	}

	rToken, unhashedTokenKey, err := h.tokenMGR.NewLoginToken(currUser.Name, userPrincipal, groupPrincipals, providerToken, ttl, description)
	return rToken, unhashedTokenKey, responseType, err
}
