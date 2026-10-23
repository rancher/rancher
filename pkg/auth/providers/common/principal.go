package common

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
)

// SplitPrincipalID parses a principal ID to get the AuthConfig name, external
// id and type.
//
// PrincipalID should look like github_[user|org|team]://12345
//
// returns configName, principalType, externalID, error
func SplitPrincipalID(principalID string) (string, string, string, error) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid principal id %v", principalID)
	}
	externalID := strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		if parts[0] == localConfigName {
			return localConfigName, "user", externalID, nil
		}
		return "", "", "", fmt.Errorf("invalid principal id %v", principalID)
	}
	if parts[0] == "" {
		return "", "", "", fmt.Errorf("invalid principal id %v", principalID)
	}

	principalType := parts[1]
	return parts[0], principalType, externalID, nil
}

// ConfigNameFromPrincipal returns the name of the AuthConfig that issued the
// principal, parsed from its ID. If the ID can't be parsed it falls back to the
// principal's provider name.
//
// UserAttribute GroupPrincipals and ExtraByProvider are keyed by this name.
func ConfigNameFromPrincipal(principal v3.Principal) string {
	configName, _, _, err := SplitPrincipalID(principal.Name)
	if err != nil {
		return principal.Provider
	}

	return configName
}

// ConfigNameFromToken returns the name of the AuthConfig that was used to generate the given token.
//
// It does this by parsing the principal ID of the user associated with the token.
//
// It returns an error if the token is invalid.
func ConfigNameFromToken(token accessor.TokenAccessor) (string, error) {
	tokenPrincipalID := token.GetUserPrincipal().Name
	configName, _, _, err := SplitPrincipalID(tokenPrincipalID)
	if err != nil {
		return "", fmt.Errorf("invalid principal: %s", tokenPrincipalID)
	}

	return configName, nil
}

// TokenHasAuthConfig reports whether the token was issued by an AuthConfig,
// i.e. it has a user principal to parse the AuthConfig name from. Most of the
// tokens Rancher issues for its own controllers, e.g. the compose and agent
// tokens, have no user principal. The autoscaler's tokens are an exception:
// they have the local principal of the autoscaler user.
func TokenHasAuthConfig(token accessor.TokenAccessor) bool {
	return token.GetUserPrincipal().Name != ""
}

// ConfigNameOrLocalFromToken returns the name of the AuthConfig that issued
// the token, falling back to local if the token has no user principal, e.g.
// the tokens Rancher issues for its own controllers.
//
// It returns an error if the token has a user principal that can't be parsed.
func ConfigNameOrLocalFromToken(token accessor.TokenAccessor) (string, error) {
	if !TokenHasAuthConfig(token) {
		return localConfigName, nil
	}

	return ConfigNameFromToken(token)
}

// ConfigNameForPrincipalLookup returns the name of the AuthConfig that issued
// the token, which must be the one that issued the principal being looked up.
//
// It returns a NotFound error if the principal was issued by another AuthConfig
// so that users of one AuthConfig can't look up the principals of another.
func ConfigNameForPrincipalLookup(principalID string, token accessor.TokenAccessor) (string, error) {
	configName, err := ConfigNameFromToken(token)
	if err != nil {
		return "", err
	}

	principalConfigName, _, _, err := SplitPrincipalID(principalID)
	if err != nil {
		return "", err
	}

	if principalConfigName != configName {
		return "", httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("principal %s was not issued by %s", principalID, configName))
	}

	return configName, nil
}
