package provider

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/tokens"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	oidcerror "github.com/rancher/rancher/pkg/oidc/provider/error"
	"github.com/rancher/rancher/pkg/oidc/provider/session"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/sirupsen/logrus"
)

const (
	supportedResponseType        = "code"
	supportedCodeChallengeMethod = "S256"
)

var supportedScopes = []string{"openid", "profile", "offline_access"}

type authParams struct {
	clientID            string
	responseType        string
	scopes              []string
	codeChallenge       string
	codeChallengeMethod string
	nonce               string
	state               string
	redirectURI         string
}

type codeCreator interface {
	GenerateCode() (string, error)
}

type sessionAdder interface {
	Add(code string, session session.Session) error
}

type authorizeHandler struct {
	tokenCache      wrangmgmtv3.TokenCache
	userLister      wrangmgmtv3.UserCache
	oidcClientCache wrangmgmtv3.OIDCClientCache
	sessionAdder    sessionAdder
	codeCreator     codeCreator
	now             func() time.Time
}

func newAuthorizeHandler(tokenCache wrangmgmtv3.TokenCache, userLister wrangmgmtv3.UserCache, sessionAdder sessionAdder, codeCreator codeCreator, oidcClientCache wrangmgmtv3.OIDCClientCache) *authorizeHandler {
	return &authorizeHandler{
		tokenCache:      tokenCache,
		userLister:      userLister,
		sessionAdder:    sessionAdder,
		codeCreator:     codeCreator,
		oidcClientCache: oidcClientCache,
		now:             time.Now,
	}
}

func (h *authorizeHandler) authEndpoint(w http.ResponseWriter, r *http.Request) {
	params, err := getAuthParamsFromRequest(r)
	if err != nil {
		oidcerror.WriteError(oidcerror.InvalidRequest, fmt.Sprintf("error parsing parameters from request %v", err), http.StatusBadRequest, w)
		return
	}
	token, err := h.getAndVerifyRancherTokenFromRequest(r)
	// redirect to the login page if the token is not present or there is any error fetching it. We need to pass all the oidc parameters from the original request.
	if err != nil {
		u, err := url.Parse(settings.ServerURL.Get() + "/dashboard/auth/login")
		if err != nil {
			oidcerror.WriteError(oidcerror.InvalidRequest, "error parsing server url", http.StatusInternalServerError, w)
			return
		}
		q := url.Values{}
		q.Set("response_type", params.responseType)
		q.Set("client_id", params.clientID)
		q.Set("redirect_uri", params.redirectURI)
		q.Set("scope", strings.Join(params.scopes, " "))
		q.Set("code_challenge", params.codeChallenge)
		q.Set("code_challenge_method", params.codeChallengeMethod)
		if params.state != "" {
			q.Set("state", params.state)
		}
		if params.nonce != "" {
			q.Set("nonce", params.nonce)
		}
		u.RawQuery = q.Encode()

		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}

	// validate all parameter as per the oidc spec.
	if params.redirectURI == "" {
		oidcerror.WriteError(oidcerror.InvalidRequest, "missing redirect_uri", http.StatusBadRequest, w)
		return
	}
	redirectURL, err := url.Parse(params.redirectURI)
	if err != nil {
		oidcerror.WriteError(oidcerror.InvalidRequest, "invalid redirect_uri", http.StatusBadRequest, w)
	}
	if redirectURL.Host == r.URL.Host && redirectURL.Path == r.URL.Path {
		oidcerror.WriteError(oidcerror.InvalidRequest, "redirect_uri can't be the same as the host uri", http.StatusBadRequest, w)
	}
	oidcClients, err := h.oidcClientCache.GetByIndex(OIDCClientByIDIndex, params.clientID)
	if err != nil {
		oidcerror.WriteError(oidcerror.InvalidRequest, fmt.Sprintf("error retrieving OIDC client: %v", err), http.StatusBadRequest, w)
		return
	}
	if len(oidcClients) == 0 {
		oidcerror.WriteError(oidcerror.InvalidRequest, fmt.Sprintf("OIDC client not found: %v", err), http.StatusBadRequest, w)
		return
	}
	if len(oidcClients) > 1 {
		oidcerror.WriteError(oidcerror.InvalidRequest, "multiple OIDC clients with the same clientID found", http.StatusBadRequest, w)
		return
	}
	oidcClient := oidcClients[0]
	if !slices.Contains(oidcClient.Spec.RedirectURIs, params.redirectURI) {
		oidcerror.WriteError(oidcerror.InvalidRequest, "redirect_uri is not registered", http.StatusBadRequest, w)
		return
	}
	// Although Access-Control-Allow-Origin is initially set in middleware based on all OIDCClients' redirectURLs,
	// we override it here because we know the exact redirectURL for this request.
	w.Header().Set("Access-Control-Allow-Origin", params.redirectURI)
	if params.responseType != supportedResponseType {
		oidcerror.RedirectWithError(params.redirectURI, oidcerror.UnsupportedResponseType, "response type not supported", params.state, w, r)
		return
	}
	if params.codeChallengeMethod != supportedCodeChallengeMethod {
		oidcerror.RedirectWithError(params.redirectURI, oidcerror.InvalidRequest, "challenge_method not supported, only S256 is supported", params.state, w, r)
		return
	}
	if !slices.Contains(params.scopes, "openid") {
		oidcerror.RedirectWithError(params.redirectURI, oidcerror.InvalidScope, "missing openid scope", params.state, w, r)
		return
	}
	for _, scope := range params.scopes {
		if !slices.Contains(supportedScopes, scope) {
			oidcerror.RedirectWithError(params.redirectURI, oidcerror.InvalidScope, fmt.Sprintf("invalid scope: %s", scope), params.state, w, r)
			return
		}
	}
	if params.codeChallenge == "" {
		oidcerror.RedirectWithError(params.redirectURI, oidcerror.InvalidRequest, "missing code_challenge", params.state, w, r)
		return
	}

	code, err := h.codeCreator.GenerateCode()
	if err != nil {
		logrus.Errorf("[OIDC provider] error generating code %v", err)
		oidcerror.RedirectWithError(params.redirectURI, oidcerror.ServerError, fmt.Sprintf("failed to generate code: %v", err), params.state, w, r)
		return
	}

	// store code and request info in a session. Session will be retrieved in the token endpoint using the code.
	err = h.sessionAdder.Add(code, session.Session{
		ClientID:      params.clientID,
		TokenName:     token.Name,
		Scope:         params.scopes,
		CodeChallenge: params.codeChallenge,
		Nonce:         params.nonce,
		CreatedAt:     h.now(),
	})
	if err != nil {
		logrus.Errorf("[OIDC provider] error adding session %v", err)
		oidcerror.RedirectWithError(params.redirectURI, oidcerror.ServerError, fmt.Sprintf("failed to store auth session: %v", err), params.state, w, r)
		return
	}

	// redirect to the redirect_uri with a valid code
	u, err := url.Parse(params.redirectURI)
	if err != nil {
		oidcerror.WriteError(oidcerror.InvalidRequest, "failed to parse redirect_uri", http.StatusBadRequest, w)
	}
	q := url.Values{}
	q.Set("code", code)
	if params.state != "" {
		q.Set("state", params.state)
	}
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h *authorizeHandler) getAndVerifyRancherTokenFromRequest(r *http.Request) (*v3.Token, error) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		return nil, fmt.Errorf("rancher token not present")
	}
	tokenName, tokenKey := tokens.SplitTokenParts(tokenAuthValue)
	if tokenName == "" || tokenKey == "" {
		return nil, fmt.Errorf("can't split rancher token")

	}
	token, err := h.tokenCache.Get(tokenName)
	if err != nil {
		// do not return early to avoid timing attacks
		fakeToken, _ := randomtoken.Generate()
		_ = subtle.ConstantTimeCompare([]byte(fakeToken), []byte(fakeToken))
		return nil, fmt.Errorf("invalid token")
	}
	// If the auth provider is specified make sure it exists and enabled.
	if token.AuthProvider != "" {
		disabled, err := providers.IsDisabledProvider(token.AuthProvider)
		if err != nil {
			return nil, fmt.Errorf("can't check if auth provider is disabled: %w", err)
		}
		if disabled {
			return nil, fmt.Errorf("auth provider is disabled")
		}
	}
	if _, err := tokens.VerifyToken(token, tokenName, tokenKey); err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}

	authUser, err := h.userLister.Get(token.UserID)
	if err != nil {
		return nil, fmt.Errorf("can't get user: %w", err)
	}

	if authUser.Enabled != nil && !*authUser.Enabled {
		return nil, fmt.Errorf("user is disabled")
	}

	return token, nil
}

// getAuthParamsFromRequest returns the params for the request. OIDC spec says that params can be either in a GET or POST request, so we should check both.
func getAuthParamsFromRequest(r *http.Request) (*authParams, error) {
	var values url.Values

	switch r.Method {
	case http.MethodGet:
		values = r.URL.Query()
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		values = r.Form
	default:
		return nil, fmt.Errorf("unsupported method")
	}

	return &authParams{
		clientID:            values.Get("client_id"),
		scopes:              strings.Split(values.Get("scope"), " "),
		codeChallenge:       values.Get("code_challenge"),
		codeChallengeMethod: values.Get("code_challenge_method"),
		nonce:               values.Get("nonce"),
		state:               values.Get("state"),
		redirectURI:         values.Get("redirect_uri"),
		responseType:        values.Get("response_type"),
	}, nil
}
