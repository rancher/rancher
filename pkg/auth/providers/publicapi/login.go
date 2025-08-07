package publicapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/rancher/rancher/pkg/auth/providers/cognito"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
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
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	CookieName = "R_SESS"
)

func newLoginHandler(mgmt *config.ScaledContext) *loginHandler {
	tokenManager := tokens.NewManager(mgmt.Wrangler)
	return &loginHandler{
		scaledContext:         mgmt,
		kubeconfigTokenGetter: tokenManager,
		ensureUser:            mgmt.UserManager.EnsureUser,
		ensureUserAttribute:   mgmt.UserManager.UserAttributeCreateOrUpdate,
		newLoginToken:         tokenManager.NewLoginToken,
	}
}

type loginHandler struct {
	scaledContext         *config.ScaledContext
	kubeconfigTokenGetter kubeconfigTokenGetter
	ensureUser            func(principalName, displayName string) (*apiv3.User, error)
	ensureUserAttribute   func(userID, provider string, groupPrincipals []apiv3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error
	newLoginToken         func(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int64, description string) (apiv3.Token, string, error)
}

type kubeconfigTokenGetter interface {
	GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal apiv3.Principal) (*apiv3.Token, string, error)
}

const (
	SAMLResponseType   = "saml"
	CookieResponseType = "cookie"
)

func (h *loginHandler) login(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName != "login" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	token, unhashedTokenKey, responseType, err := h.createLoginToken(request)
	if err != nil {
		// If the user fails to authenticate, hide the details of the exact error.
		// Bad credentials will already be APIErrors. Otherwise, return a generic error message.
		if httperror.IsAPIError(err) {
			return err
		}

		return httperror.WrapAPIError(err, httperror.ServerError, "Server error while authenticating")
	}

	switch responseType {
	case CookieResponseType:
		tokenCookie := &http.Cookie{
			Name:     CookieName,
			Value:    token.ObjectMeta.Name + ":" + unhashedTokenKey,
			Secure:   true,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(request.Response, tokenCookie)
	case SAMLResponseType: // Do nothing.
	default:
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
func (h *loginHandler) createLoginToken(request *types.APIContext) (apiv3.Token, string, string, error) {
	logrus.Debugf("Creating a login token")

	bytes, err := io.ReadAll(request.Request.Body)
	if err != nil {
		logrus.Errorf("Error reading request body: %v", err)
		return apiv3.Token{}, "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	generic := &apiv3.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("Error unmarshalling GenericLogin: %v", err)
		return apiv3.Token{}, "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}
	responseType := generic.ResponseType
	description := generic.Description
	ttl := generic.TTLMillis

	authTimeout := settings.AuthUserSessionTTLMinutes.Get()
	if minutes, err := strconv.ParseInt(authTimeout, 10, 64); err == nil {
		ttl = minutes * 60 * 1000
	}

	var (
		input          any
		providerName   string
		isSAMLProvider bool
	)
	switch request.Type {
	case client.LocalProviderType:
		input = &apiv3.BasicLogin{}
		providerName = local.Name
	case client.GithubProviderType:
		input = &apiv3.GithubLogin{}
		providerName = github.Name
	case client.GithubAppProviderType:
		input = &apiv3.GithubLogin{}
		providerName = githubapp.Name
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
		isSAMLProvider = true
	case client.ADFSProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.ADFSName
		isSAMLProvider = true
	case client.KeyCloakProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.KeyCloakName
		isSAMLProvider = true
	case client.OKTAProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.OKTAName
		isSAMLProvider = true
	case client.ShibbolethProviderType:
		input = &apiv3.SamlLoginInput{}
		providerName = saml.ShibbolethName
		isSAMLProvider = true
	case client.GoogleOAuthProviderType:
		input = &apiv3.GoogleOauthLogin{}
		providerName = googleoauth.Name
	case client.OIDCProviderType:
		input = &apiv3.OIDCLogin{}
		providerName = oidc.Name
	case client.KeyCloakOIDCProviderType:
		input = &apiv3.OIDCLogin{}
		providerName = keycloakoidc.Name
	case client.GenericOIDCProviderType:
		input = &apiv3.OIDCLogin{}
		providerName = genericoidc.Name
	case client.CognitoProviderType:
		input = &apiv3.OIDCLogin{}
		providerName = cognito.Name
	default:
		return apiv3.Token{}, "", "", httperror.NewAPIError(httperror.ServerError, "unknown authentication provider")
	}

	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("Error unmarshalling %T: %v", input, err)
		return apiv3.Token{}, "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	if isSAMLProvider {
		// SAML's login flow is different. Unlike other providers it gets the logged in user's data
		// via a POST from the identity provider on a separate endpoint.
		err = saml.PerformSamlLogin(providerName, request, input)
		return apiv3.Token{}, "", SAMLResponseType, err
	}

	ctx := context.WithValue(request.Request.Context(), util.RequestKey, request.Request)
	userPrincipal, groupPrincipals, providerToken, err := providers.AuthenticateUser(ctx, input, providerName)
	if err != nil {
		return apiv3.Token{}, "", "", err
	}

	displayName := userPrincipal.DisplayName
	if displayName == "" {
		displayName = userPrincipal.LoginName
	}

	var (
		user    *apiv3.User
		backoff = wait.Backoff{
			Duration: 100 * time.Millisecond,
			Factor:   2,
			Jitter:   .2,
			Steps:    5,
		}
	)

	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error

		user, err = h.ensureUser(userPrincipal.Name, displayName)
		if err != nil {
			logrus.Warnf("Error creating or updating user for %s, retrying: %v", userPrincipal.Name, err)
			return false, nil
		}

		if !user.GetEnabled() {
			return true, nil
		}

		loginTime := time.Now()
		userExtraInfo := providers.GetUserExtraAttributes(providerName, userPrincipal)
		err = h.ensureUserAttribute(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo, loginTime)
		if err != nil {
			logrus.Warnf("Error creating or updating userAttribute for %s, retrying: %v", userPrincipal.Name, err)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return apiv3.Token{}, "", "", fmt.Errorf("error creating or updating user and/or userAttribute for %s: %w", userPrincipal.Name, err)
	}

	if !user.GetEnabled() {
		return apiv3.Token{}, "", "", httperror.NewAPIError(httperror.PermissionDenied, "Permission Denied")
	}

	if strings.HasPrefix(responseType, tokens.KubeconfigResponseType) {
		token, tokenKey, err := tokens.GetKubeConfigToken(user.Name, responseType, h.kubeconfigTokenGetter, userPrincipal)
		if err != nil {
			return apiv3.Token{}, "", "", err
		}

		return *token, tokenKey, responseType, nil
	}

	token, tokenKey, err := h.newLoginToken(user.Name, userPrincipal, groupPrincipals, providerToken, ttl, description)
	if err != nil {
		logrus.Errorf("Error creating login token for user %s: %v", user.Name, err)
		return apiv3.Token{}, "", "", err
	}

	return token, tokenKey, responseType, err
}
