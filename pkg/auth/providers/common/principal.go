package common

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/auth/accessor"
)

// SplitPrincipalID parses a principal ID to get the provider, external id and
// type.
//
// PrincipalID should look like looks like github_[user|org|team]://12345
//
// returns provider, principalType, externalID, error
func SplitPrincipalID(principalID string) (string, string, string, error) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid principal id %v", principalID)
	}
	externalID := strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid principal id %v", principalID)
	}

	principalType := parts[1]
	return parts[0], principalType, externalID, nil
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
