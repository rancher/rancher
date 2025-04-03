package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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

type userInfoHandler struct {
	userCache           wrangmgmtv3.UserCache
	userAttributeLister wrangmgmtv3.UserAttributeCache
	jwks                signingKeyGetter
}

func newUserInfoHandler(userLister wrangmgmtv3.UserCache, userAttributeLister wrangmgmtv3.UserAttributeCache, jwks signingKeyGetter) *userInfoHandler {

	return &userInfoHandler{
		userCache:           userLister,
		userAttributeLister: userAttributeLister,
		jwks:                jwks,
	}
}

// userInfoEndpoint handles the userinfo endpoint of the OIDC provider
func (h *userInfoHandler) userInfoEndpoint(w http.ResponseWriter, r *http.Request) {
	accessToken, err := getTokenFromHeader(r)
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

	attribs, err := h.userAttributeLister.Get(userId)
	if err != nil && !apierrors.IsNotFound(err) {
		oidcerror.WriteError(oidcerror.ServerError, fmt.Sprintf("can't get user attributes: %v", err), http.StatusInternalServerError, w)
		return
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

	if groups != nil {
		response.Groups = groups
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&response); err != nil {
		http.Error(w, "failed to encode user info response", http.StatusInternalServerError)
	}

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
