package provider

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	oidcerror "github.com/rancher/rancher/pkg/oidc/provider/error"
	"github.com/rancher/rancher/pkg/oidc/provider/session"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang-jwt/jwt/v5"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/tokens"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type sessionGetterRemover interface {
	Get(code string) (*session.Session, error)
	Remove(code string) error
}

type signingKeyGetter interface {
	GetSigningKey() (*rsa.PrivateKey, string, error)
	GetPublicKey(kid string) (*rsa.PublicKey, error)
}

type tokenHandler struct {
	tokenCache          wrangmgmtv3.TokenCache
	tokenClient         wrangmgmtv3.TokenClient
	userLister          wrangmgmtv3.UserCache
	userAttributeLister wrangmgmtv3.UserAttributeCache
	sessionClient       sessionGetterRemover
	oidcClientCache     wrangmgmtv3.OIDCClientCache
	oidcClient          wrangmgmtv3.OIDCClientClient
	secretCache         corev1.SecretCache
	oidcClientIndexer   cache.Indexer
	jwks                signingKeyGetter
	now                 func() time.Time
}

// TokenResponse represents a successful response returned by the token endpoint
type TokenResponse struct {
	// IDToken is the oidc token generated.
	IDToken string `json:"id_token"`
	// AccessToken is the access token generated.
	AccessToken string `json:"access_token"`
	// AccessToken is the refresh token generated.
	RefreshToken string `json:"refresh_token,omitempty"`
	// ExpiresIn indicates when id_token and access_token expire.
	ExpiresIn int `json:"expires_in"`
}

// RefreshTokenClaims represent claims in the refresh_token
type RefreshTokenClaims struct {
	jwt.RegisteredClaims
	// RancherTokenHash is the hash of the Rancher token used for refreshing the token.
	RancherTokenHash string `json:"rancher_token_hash"`
	// Scope indicates the scopes for this token.
	Scope []string `json:"scope"`
}

func newTokenHandler(tokenCache wrangmgmtv3.TokenCache,
	userLister wrangmgmtv3.UserCache,
	userAttributeLister wrangmgmtv3.UserAttributeCache,
	sessionClient sessionGetterRemover,
	jwks signingKeyGetter,
	oidcClientCache wrangmgmtv3.OIDCClientCache,
	oidcClient wrangmgmtv3.OIDCClientClient,
	secretCache corev1.SecretCache,
	tokenClient wrangmgmtv3.TokenClient) *tokenHandler {

	return &tokenHandler{
		tokenCache:          tokenCache,
		tokenClient:         tokenClient,
		userLister:          userLister,
		userAttributeLister: userAttributeLister,
		sessionClient:       sessionClient,
		jwks:                jwks,
		oidcClientCache:     oidcClientCache,
		oidcClient:          oidcClient,
		secretCache:         secretCache,
		now:                 time.Now,
	}
}

// tokenEndpoint handles the token endpoint of the OIDC provider
func (h *tokenHandler) tokenEndpoint(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		logrus.Debug("[OIDC provider] error parsing request  form values")
		oidcerror.WriteError(oidcerror.InvalidRequest, fmt.Sprintf("error parsing parameters from request %v", err), http.StatusBadRequest, w)
		return
	}

	switch r.Form.Get("grant_type") {
	case "authorization_code":
		tokenResponse, oidcErr := h.createTokenFromCode(r)
		if oidcErr != nil {
			logrus.Debug("[OIDC provider] error creating token response: " + oidcErr.ToString())
			oidcErr.Write(http.StatusBadRequest, w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(tokenResponse)
		if err != nil {
			oidcerror.WriteError(oidcerror.ServerError, "failed to encode token response", http.StatusInternalServerError, w)
			return
		}
	case "refresh_token":
		tokenResponse, oidcErr := h.createRefreshToken(r)
		if oidcErr != nil {
			logrus.Debug("[OIDC provider] error creating refresh token response: " + oidcErr.ToString())
			oidcErr.Write(http.StatusBadRequest, w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(tokenResponse)
		if err != nil {
			oidcerror.WriteError(oidcerror.ServerError, "failed to encode refresh token response", http.StatusInternalServerError, w)
			return
		}
	default:
		http.Error(w, "grant_type not supported", http.StatusInternalServerError)
		return
	}
}

// createTokenFromCode creates a response with an id_token, access_token and refresh_token
func (h *tokenHandler) createTokenFromCode(r *http.Request) (TokenResponse, *oidcerror.Error) {
	code := r.FormValue("code")
	session, err := h.sessionClient.Get(code)
	if err != nil {
		if errors.IsNotFound(err) {
			return TokenResponse{}, oidcerror.New(oidcerror.InvalidRequest, "invalid code")
		}
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, "error retrieving session :"+err.Error())
	}

	// verify clientID and secret. They can be set in the Authorization header or as a form param as specified in the OIDC spec.
	var clientID, _ string
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}
	if clientID != session.ClientID {
		return TokenResponse{}, oidcerror.New(oidcerror.InvalidRequest, "invalid client_id")
	}
	oidcClient, err := h.getOIDCClientByClientID(clientID)
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, "failed to get OIDC client")
	}
	secret, err := h.secretCache.Get(secretsNamespace, clientID)
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, "failed to get client secret")
	}
	clientSecretFound := false
	for key, cs := range secret.Data {
		if clientSecret == string(cs) {
			clientSecretFound = true
			if err := h.updateClientSecretUsedTimeStamp(oidcClient, key); err != nil {
				return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to add OIDC Client ID to Rancher token: %v", err))
			}
			break
		}
	}
	if !clientSecretFound {
		return TokenResponse{}, oidcerror.New(oidcerror.InvalidRequest, "invalid client_secret")
	}

	// PKCE verification
	code_verifier := r.Form.Get("code_verifier")
	if session.CodeChallenge != oauth2.S256ChallengeFromVerifier(code_verifier) {
		return TokenResponse{}, oidcerror.New(oidcerror.InvalidRequest, "failed to verify PKCE code challenge")
	}

	rancherToken, err := h.tokenCache.Get(session.TokenName)
	if err != nil {
		if errors.IsNotFound(err) {
			return TokenResponse{}, oidcerror.New(oidcerror.InvalidRequest, "Rancher token is not valid anymore")
		}
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, "failed to get Rancher token: "+err.Error())
	}
	resp, oidcErr := h.createTokenResponse(rancherToken, oidcClient, session.Nonce, session.Scope)
	if oidcErr == nil {
		err := h.sessionClient.Remove(code)
		if err != nil && !errors.IsNotFound(err) {
			logrus.Warnf("[OIDC provider] error removing session: %v", err)
		}
	}

	return resp, oidcErr
}

// createRefreshToken issues a new id_token, access_token and refresh_token using a refresh_token
func (h *tokenHandler) createRefreshToken(r *http.Request) (TokenResponse, *oidcerror.Error) {
	refreshToken := r.Form.Get("refresh_token")
	// verify refresh_token signature
	token, err := jwt.ParseWithClaims(refreshToken, &RefreshTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure correct signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("can't find kid")
		}
		pubKey, err := h.jwks.GetPublicKey(kid)
		if err != nil {
			return nil, err
		}

		return pubKey, nil
	})
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to parse refresh token: %v", err))
	}
	claims, ok := token.Claims.(*RefreshTokenClaims)
	if !ok && !token.Valid {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, "refresh token not valid")
	}

	// get rancher Token associated with this refresh_token
	tokenList, err := h.tokenCache.List(labels.SelectorFromSet(map[string]string{
		tokens.UserIDLabel: claims.Subject,
	}))
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to add OIDC Client ID to Rancher token: %v", err))
	}
	var rancherToken *v3.Token
	for _, token := range tokenList {
		hash := sha256.Sum256([]byte(token.Name))
		rancherTokenHash := hex.EncodeToString(hash[:])
		if rancherTokenHash == claims.RancherTokenHash {
			rancherToken = token
			break
		}
	}
	if rancherToken == nil {
		return TokenResponse{}, oidcerror.New(oidcerror.AccessDenied, "Rancher token no longer present.")
	}

	// identify the OIDC client for the refresh_token using the audience
	if len(claims.Audience) < 1 {
		return TokenResponse{}, oidcerror.New(oidcerror.InvalidRequest, "can't find client in audience")
	}
	oidcClient, err := h.getOIDCClientByClientID(claims.Audience[0])
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to get oidc client: %v", err))
	}

	return h.createTokenResponse(rancherToken, oidcClient, "", claims.Scope)
}

// createTokenResponse creates an id_token, access_token and refresh_token for a valid Rancher token
func (h *tokenHandler) createTokenResponse(rancherToken *v3.Token, oidcClient *v3.OIDCClient, nonce string, scopes []string) (TokenResponse, *oidcerror.Error) {
	// verify Rancher token and user are valid
	if rancherToken.Expired {
		return TokenResponse{}, oidcerror.New(oidcerror.AccessDenied, "Rancher token is expired")
	}
	if rancherToken.Enabled != nil && !*rancherToken.Enabled {
		return TokenResponse{}, oidcerror.New(oidcerror.AccessDenied, "Rancher token is disabled")
	}
	if rancherToken.AuthProvider != "" {
		disabled, err := providers.IsDisabledProvider(rancherToken.AuthProvider)
		if err != nil {
			return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("can't check if auth provider is disabled: %v", err))
		}
		if disabled {
			return TokenResponse{}, oidcerror.New(oidcerror.AccessDenied, "auth provider is disabled")
		}
	}
	user, err := h.userLister.Get(rancherToken.UserID)
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("can't get user: %v", err))
	}
	if user.Enabled != nil && !*user.Enabled {
		return TokenResponse{}, oidcerror.New(oidcerror.AccessDenied, "user is disabled")
	}
	attribs, err := h.userAttributeLister.Get(rancherToken.UserID)
	if err != nil && !apierrors.IsNotFound(err) {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("can't get user attributes: %v", err))
	}
	var groups []string
	if attribs != nil {
		for _, gps := range attribs.GroupPrincipals {
			for _, principal := range gps.Items {
				name := strings.TrimPrefix(principal.Name, "local://")
				groups = append(groups, name)
			}
		}
	}

	key, kid, err := h.jwks.GetSigningKey()
	if err != nil {
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to get signing key: %v", err))
	}
	// create id_token
	idClaims := jwt.MapClaims{
		"aud": []string{oidcClient.Status.ClientID},
		"exp": h.now().Add(oidcClient.Spec.TokenExpirationSeconds * time.Second).Unix(),
		"iss": settings.ServerURL.Get() + "/oidc",
		"iat": h.now().Unix(),
		"sub": rancherToken.UserID,
	}
	if slices.Contains(scopes, "profile") {
		idClaims["preferred_username"] = user.DisplayName
	}
	if nonce != "" {
		idClaims["nonce"] = nonce
	}
	if groups != nil {
		idClaims["groups"] = groups
	}
	if rancherToken.AuthProvider != "" {
		idClaims["auth_provider"] = rancherToken.AuthProvider
	}
	idToken := jwt.NewWithClaims(jwt.SigningMethodRS256, idClaims)
	idToken.Header["kid"] = kid
	idTokenString, err := idToken.SignedString(key)
	if err != nil {
		logrus.Errorf("[OIDC provider] failed to sign id token %v", err)
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to sign id token: %v", err))
	}

	// create access_token
	accessClaims := jwt.MapClaims{
		"aud":   []string{oidcClient.Status.ClientID},
		"exp":   h.now().Add(oidcClient.Spec.TokenExpirationSeconds * time.Second).Unix(),
		"iss":   settings.ServerURL.Get() + "/oidc",
		"iat":   h.now().Unix(),
		"sub":   rancherToken.UserID,
		"scope": scopes,
	}
	if rancherToken.AuthProvider != "" {
		accessClaims["auth_provider"] = rancherToken.AuthProvider
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessToken.Header["kid"] = kid

	accessTokenString, err := accessToken.SignedString(key)
	if err != nil {
		logrus.Errorf("[OIDC provider] failed to sign access token %v", err)
		return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to sign access token: %v", err))
	}
	resp := TokenResponse{
		IDToken:     idTokenString,
		AccessToken: accessTokenString,
	}

	// create refresh_token
	if slices.Contains(scopes, "offline_access") {
		hash := sha256.Sum256([]byte(rancherToken.Name))
		rancherTokenHash := hex.EncodeToString(hash[:])
		refreshClaims := jwt.MapClaims{
			"aud":                []string{oidcClient.Status.ClientID},
			"exp":                h.now().Add(oidcClient.Spec.RefreshTokenExpirationSeconds * time.Second).Unix(),
			"iat":                h.now().Unix(),
			"sub":                rancherToken.UserID,
			"rancher_token_hash": rancherTokenHash,
			"scope":              scopes,
		}
		if rancherToken.AuthProvider != "" {
			refreshClaims["auth_provider"] = rancherToken.AuthProvider
		}
		refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
		refreshToken.Header["kid"] = kid
		refreshTokenString, err := refreshToken.SignedString(key)
		if err != nil {
			logrus.Errorf("[OIDC provider] failed to sign refresh token %v", err)
			return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to sign refresh token: %v", err))
		}
		resp.RefreshToken = refreshTokenString

		if err := h.addOIDCClientIDToRancherToken(oidcClient.Name, rancherToken.Name); err != nil {
			return TokenResponse{}, oidcerror.New(oidcerror.ServerError, fmt.Sprintf("failed to add OIDC Client ID to Rancher token: %v", err))
		}
	}

	resp.ExpiresIn = int(oidcClient.Spec.TokenExpirationSeconds * time.Second)

	return resp, nil
}

func (h *tokenHandler) updateClientSecretUsedTimeStamp(oidcClient *v3.OIDCClient, clientSecretID string) interface{} {
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:    "add",
		Path:  "/metadata/annotations/cattle.io.oidc-client-secret-used-" + clientSecretID,
		Value: metav1.NewTime(h.now()),
	}})
	if err != nil {
		return err
	}

	_, err = h.oidcClient.Patch(oidcClient.Name, types.JSONPatchType, patch)

	return err
}

func (h *tokenHandler) addOIDCClientIDToRancherToken(oidcClientName string, rancherTokenName string) error {
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:    "add",
		Path:  "/metadata/labels/cattle.io.oidc-client-" + oidcClientName,
		Value: "true",
	}})
	if err != nil {
		return err
	}
	_, err = h.tokenClient.Patch(rancherTokenName, types.JSONPatchType, patch)

	return err
}

func (h *tokenHandler) getOIDCClientByClientID(clientID string) (*v3.OIDCClient, error) {
	oidcClients, err := h.oidcClientCache.GetByIndex(oidcClientByIDIndex, clientID)
	if err != nil {
		return nil, fmt.Errorf("error retreiving OIDC client: %w", err)
	}
	if len(oidcClients) == 0 {
		return nil, fmt.Errorf("no OIDC clients found")
	}
	return oidcClients[0], nil
}
