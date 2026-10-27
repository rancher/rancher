package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	oidcerror "github.com/rancher/rancher/pkg/oidc/provider/error"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// UserInfoResponse represents the response from the userinfo endpoint.
type UserInfoResponse struct {
	Sub      string   `json:"sub"`
	UserName string   `json:"username,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// rancherTokenFetcher fetches a v3 or ext Rancher token by name.
type rancherTokenFetcher interface {
	Fetch(name string) (accessor.TokenAccessor, error)
}

type userInfoHandler struct {
	userCache           wrangmgmtv3.UserCache
	userAttributeLister wrangmgmtv3.UserAttributeCache
	jwks                signingKeyGetter
	tokenFetcher        rancherTokenFetcher
}

func newUserInfoHandler(userLister wrangmgmtv3.UserCache, userAttributeLister wrangmgmtv3.UserAttributeCache, jwks signingKeyGetter, tokenFetcher rancherTokenFetcher) *userInfoHandler {

	return &userInfoHandler{
		userCache:           userLister,
		userAttributeLister: userAttributeLister,
		jwks:                jwks,
		tokenFetcher:        tokenFetcher,
	}
}

// userInfoEndpoint handles the userinfo endpoint of the OIDC provider
func (h *userInfoHandler) userInfoEndpoint(w http.ResponseWriter, r *http.Request) {
	accessToken, err := getTokenFromHeader(r)
	if err != nil {
		oidcerror.WriteError(oidcerror.InvalidRequest, fmt.Sprintf("failed to get token from header: %v", err), http.StatusBadRequest, w)
		return
	}

	claims := jwt.MapClaims{}
	// verify access_token signature
	_, err = jwt.ParseWithClaims(accessToken, &claims, func(token *jwt.Token) (interface{}, error) {
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
		oidcerror.WriteError(oidcerror.InvalidRequest, fmt.Sprintf("invalid access_token: %v", err), http.StatusBadRequest, w)
		return
	}

	userId, ok := claims["sub"].(string)
	if !ok {
		oidcerror.WriteError(oidcerror.InvalidRequest, "invalid access_token: it doesn't have sub", http.StatusBadRequest, w)
		return
	}
	scopes, ok := claims["scope"].([]interface{})
	if !ok {
		oidcerror.WriteError(oidcerror.InvalidRequest, "invalid access_token: it doesn't have scope", http.StatusBadRequest, w)
		return
	}
	response := UserInfoResponse{
		Sub: userId,
	}
	if slices.Contains(scopes, "profile") {
		user, err := h.userCache.Get(userId)
		if err != nil {
			oidcerror.WriteError(oidcerror.ServerError, fmt.Sprintf("can't get user: %v", err), http.StatusInternalServerError, w)
			return
		}
		response.UserName = user.DisplayName
	}

	if slices.Contains(scopes, "groups") {
		attribs, err := h.userAttributeLister.Get(userId)
		if err != nil && !apierrors.IsNotFound(err) {
			oidcerror.WriteError(oidcerror.ServerError, fmt.Sprintf("can't get user attributes: %v", err), http.StatusInternalServerError, w)
			return
		}
		configName, _ := claims[configNameClaim].(string)
		if configName == "" {
			configName, err = h.configNameFromRancherToken(claims, userId)
			if err != nil {
				oidcerror.WriteError(oidcerror.ServerError, fmt.Sprintf("can't get rancher token: %v", err), http.StatusInternalServerError, w)
				return
			}
		}
		groups := groupsForConfig(attribs, configName)
		if groups != nil {
			response.Groups = groups
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if err := json.NewEncoder(w).Encode(&response); err != nil {
		http.Error(w, "failed to encode user info response", http.StatusInternalServerError)
	}

}

// configNameFromRancherToken returns the name of the AuthConfig that issued
// the Rancher token named in the token claim, for access tokens issued before
// the config_name claim was added. Their deprecated auth_provider claim holds
// the provider type, which several AuthConfigs can share, so it can't be used.
//
// It returns "" if the JWT names no Rancher token, e.g. a refresh token, or if
// the Rancher token no longer exists or belongs to another user, so that these
// get no groups rather than the groups of the wrong AuthConfig.
func (h *userInfoHandler) configNameFromRancherToken(claims jwt.MapClaims, userID string) (string, error) {
	tokenName, _ := claims["token"].(string)
	if tokenName == "" {
		return "", nil
	}

	token, err := h.tokenFetcher.Fetch(tokenName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if token.GetUserID() != userID {
		return "", nil
	}

	configName, err := common.ConfigNameOrLocalFromToken(token)
	if err != nil {
		return "", nil
	}

	return configName, nil
}

func getTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is missing")
	}

	// Expected format: "Bearer <token>"
	_, token, found := strings.Cut(authHeader, "Bearer ")
	if !found {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return token, nil
}
