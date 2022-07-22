package clients

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/go-autorest/autorest/adal"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

// GetPrincipalID attempts to extract the ID of either a user or group from the principal value.
func GetPrincipalID(principal v3.Principal) string {
	name := principal.ObjectMeta.Name
	if parts := strings.Split(name, "//"); len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// ExtractFieldFromJWT attempts to extract a value from a JWT by field name.
// It does not make assumptions about the type of the return value, so the caller is responsible for handling that.
func ExtractFieldFromJWT(tokenString string, fieldID string) (interface{}, error) {
	pieces := strings.Split(tokenString, ".")
	if len(pieces) != 3 {
		return "", fmt.Errorf("invalid token")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(pieces[1])
	if err != nil {
		return "", fmt.Errorf("error decoding token")
	}

	var data map[string]interface{}
	err = json.Unmarshal(decoded, &data)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling token")
	}
	if _, ok := data[fieldID]; !ok {
		return "", fmt.Errorf("missing field %s from token", fieldID)
	}
	return data[fieldID], nil
}

// ParsePrincipalID accepts a principalID in the format <provider>_<type>://<ID>
// and returns a map containing ID, provider, and type.
func ParsePrincipalID(principalID string) (map[string]string, error) {
	parsed := make(map[string]string)

	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return parsed, fmt.Errorf("invalid id %v", principalID)
	}
	externalID := strings.TrimPrefix(parts[1], "//")

	parsed["ID"] = externalID

	pparts := strings.SplitN(parts[0], "_", 2)
	if len(pparts) != 2 {
		return parsed, fmt.Errorf("invalid id %v", principalID)
	}

	parsed["provider"] = pparts[0]
	parsed["type"] = pparts[1]

	return parsed, nil
}

func unmarshalADALToken(secret string) (adal.Token, error) {
	var azureToken adal.Token
	err := json.Unmarshal([]byte(secret), &azureToken)
	if err != nil {
		return azureToken, err
	}
	return azureToken, nil
}
