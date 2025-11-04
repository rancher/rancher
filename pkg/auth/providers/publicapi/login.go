package publicapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/apiserver/pkg/apierror"
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
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
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

type kubeconfigTokenGetter interface {
	GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal apiv3.Principal) (*apiv3.Token, string, error)
}

type loginHandler struct {
	scaledContext         *config.ScaledContext
	kubeconfigTokenGetter kubeconfigTokenGetter
	ensureUser            func(principalName, displayName string) (*apiv3.User, error)
	ensureUserAttribute   func(userID, provider string, groupPrincipals []apiv3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error
	newLoginToken         func(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int64, description string) (*apiv3.Token, string, error)
}

func newV1LoginHandler(scaledContext *config.ScaledContext) *v1LoginHandler {
	return &v1LoginHandler{
		h: newLoginHandler(scaledContext),
	}
}

type v1LoginHandler struct {
	h *loginHandler
}

func (h *v1LoginHandler) login(w http.ResponseWriter, r *http.Request) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("login: Error reading request body: %s", err)
		util.ReturnAPIError(w, apierror.NewAPIError(validation.InvalidBodyContent, ""))
		return
	}

	generic := &apiv3.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("login: Error unmarshalling generic login request: %s", err)
		util.ReturnAPIError(w, apierror.NewAPIError(validation.InvalidBodyContent, ""))
		return
	}

	input := providerInputForType(generic.Type)
	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("login: Error unmarshalling provider specific login request %T: %s", input, err)
		util.ReturnAPIError(w, apierror.NewAPIError(validation.InvalidBodyContent, ""))
		return
	}

	h.h.login(w, r, input)
}

func newV3LoginHandler(scaledContext *config.ScaledContext) *v3LoginHandler {
	return &v3LoginHandler{
		h: newLoginHandler(scaledContext),
	}
}

type v3LoginHandler struct {
	h *loginHandler
}

func (h *v3LoginHandler) login(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName != "login" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	bytes, err := io.ReadAll(request.Request.Body)
	if err != nil {
		logrus.Errorf("login: Error reading request body: %s", err)
		return httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	generic := &apiv3.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("login: Error unmarshalling generic login request: %s", err)
		return httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	input := providerInputForType(request.Type)
	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("login: Error unmarshalling provider specific login request %T: %s", input, err)
		return httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	h.h.login(request.Response, request.Request, input)
	return nil
}

const (
	SAMLResponseType   = "saml"
	CookieResponseType = "cookie"
)

type loginAccessor interface {
	GetType() string
	GetTTL() int64
	GetDescription() string
	GetResponseType() string
	GetName() string
}

func providerInputForType(providerType string) loginAccessor {
	switch providerType {
	case client.LocalProviderType:
		return &apiv3.BasicLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: local.Name},
		}
	case client.GithubProviderType:
		return &apiv3.GithubLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: github.Name},
		}
	case client.GithubAppProviderType:
		return &apiv3.GithubLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: githubapp.Name},
		}
	case client.ActiveDirectoryProviderType:
		return &apiv3.BasicLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: activedirectory.Name},
		}
	case client.AzureADProviderType:
		return &apiv3.AzureADLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: azure.Name},
		}
	case client.OpenLdapProviderType:
		return &apiv3.BasicLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: ldap.OpenLdapName},
		}
	case client.FreeIpaProviderType:
		return &apiv3.BasicLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: ldap.FreeIpaName},
		}
	case client.PingProviderType:
		return &apiv3.SamlLoginInput{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: saml.PingName},
		}
		// isSAMLProvider = true
	case client.ADFSProviderType:
		return &apiv3.SamlLoginInput{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: saml.ADFSName},
		}
		// isSAMLProvider = true
	case client.KeyCloakProviderType:
		return &apiv3.SamlLoginInput{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: saml.KeyCloakName},
		}
		// isSAMLProvider = true
	case client.OKTAProviderType:
		return &apiv3.SamlLoginInput{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: saml.OKTAName},
		}
		// isSAMLProvider = true
	case client.ShibbolethProviderType:
		return &apiv3.SamlLoginInput{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: saml.ShibbolethName},
		}
		// isSAMLProvider = true
	case client.GoogleOAuthProviderType:
		return &apiv3.GoogleOauthLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: googleoauth.Name},
		}
	case client.OIDCProviderType:
		return &apiv3.OIDCLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: oidc.Name},
		}
	case client.KeyCloakOIDCProviderType:
		return &apiv3.OIDCLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: keycloakoidc.Name},
		}
	case client.GenericOIDCProviderType:
		return &apiv3.OIDCLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: genericoidc.Name},
		}
	case client.CognitoProviderType:
		return &apiv3.OIDCLogin{
			GenericLogin: apiv3.GenericLogin{Type: providerType, Name: cognito.Name},
		}
	default:
		return nil
	}
}

func (h *loginHandler) login(w http.ResponseWriter, r *http.Request, input loginAccessor) {
	if input == nil {
		logrus.Errorf("login: missing auth provider input")
		util.ReturnAPIError(w, apierror.NewAPIError(validation.InvalidBodyContent, ""))
		return
	}

	if providers.IsSAMLProviderType(input.GetType()) {
		// SAML's login flow is different. Unlike other providers it gets the logged in user's data
		// via the POST from the identity provider on a separate endpoint.
		err := saml.PerformSamlLogin(r, w, input.GetName(), input)
		if err != nil {
			if !util.IsAPIError(err) {
				logrus.Errorf("login: Error performing SAML login: %s", err)
			}
			util.ReturnAPIError(w, err)
		}
		return
	}

	userPrincipal, groupPrincipals, providerToken, err := providers.AuthenticateUser(w, r, input, input.GetName())
	if err != nil {
		if !util.IsAPIError(err) {
			logrus.Errorf("login: Error authenticating user: %s", err)
		}
		util.ReturnAPIError(w, err)
		return
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
			logrus.Warnf("login: Error creating or updating user for %s, retrying: %s", userPrincipal.Name, err)
			return false, nil
		}

		if !user.GetEnabled() {
			return true, nil
		}

		loginTime := time.Now()
		userExtraInfo := providers.GetUserExtraAttributes(input.GetName(), userPrincipal)
		err = h.ensureUserAttribute(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo, loginTime)
		if err != nil {
			logrus.Warnf("login: Error creating or updating userAttribute for %s, retrying: %s", userPrincipal.Name, err)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		logrus.Errorf("login: Error creating or updating user and/or userAttribute for %s: %s", userPrincipal.Name, err)
		util.ReturnAPIError(w, apierror.NewAPIError(validation.ServerError, ""))
		return

	}

	if !user.GetEnabled() {
		util.ReturnAPIError(w, apierror.NewAPIError(validation.PermissionDenied, ""))
		return
	}

	responseType := input.GetResponseType()
	description := input.GetDescription()
	ttl := input.GetTTL()

	authTimeout := settings.AuthUserSessionTTLMinutes.Get()
	if minutes, err := strconv.ParseInt(authTimeout, 10, 64); err == nil {
		ttl = minutes * 60 * 1000
	}

	var (
		token    *apiv3.Token
		tokenKey string
	)
	if strings.HasPrefix(responseType, tokens.KubeconfigResponseType) {
		token, tokenKey, err = tokens.GetKubeConfigToken(user.Name, responseType, h.kubeconfigTokenGetter, userPrincipal)
		if err != nil {
			logrus.Errorf("login: Error generating kubeconfig token: %s", err)
			util.ReturnAPIError(w, apierror.NewAPIError(validation.ServerError, ""))
			return
		}
	} else {
		token, tokenKey, err = h.newLoginToken(user.Name, userPrincipal, groupPrincipals, providerToken, ttl, description)
		if err != nil {
			logrus.Errorf("login: Error creating login token for user %s: %v", user.Name, err)
			util.ReturnAPIError(w, apierror.NewAPIError(validation.ServerError, ""))
			return
		}
	}

	bearerToken := token.Name + ":" + tokenKey

	if responseType == CookieResponseType {
		tokenCookie := &http.Cookie{
			Name:     CookieName,
			Value:    bearerToken,
			Secure:   true,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)

		return
	}

	token = token.DeepCopy()
	tokens.SetTokenExpiresAt(token)

	// Only return details that are actually used.
	tokenData := map[string]any{
		"token":     bearerToken,
		"expiresAt": token.ExpiresAt,
		// The following fields are included for backwards compatibility
		// with existing v3 clients e.g. the Rancher terraform provider.
		"id":       token.Name,
		"baseType": "token",
		"type":     "token",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(tokenData)
	if err != nil {
		logrus.Errorf("login: Error writing response: %v", err)
		return
	}
}
