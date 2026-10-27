package scimconfig

import "strings"

// UserNameKey is the UserAttribute extra key SCIM stores its userName under. Login and the provider
// refresher write the login name under "username", so SCIM keeps its own key.
const UserNameKey = "scimusername"

// UserIdentity returns the SCIM userName and externalId of a user from the user's extras for
// provider. The values SCIM wrote come first. When SCIM's ID attribute (cfg.UserIDAttribute) isn't
// stored, it's read from the user's <provider>_user:// principal, which is built from that
// attribute. The "username" extra comes last: login writes the login name there, and data written
// before SCIM had its own key can hold SCIM's userName.
func UserIdentity(extras map[string][]string, provider string, cfg Config) (userName, externalID string) {
	principalValue, ok := strings.CutPrefix(firstValue(extras["principalid"]), provider+"_user://")
	if !ok {
		principalValue = ""
	}

	scimUserName := firstValue(extras[UserNameKey])
	scimExternalID := firstValue(extras["externalid"])
	loginName := firstValue(extras["username"])

	switch cfg.UserIDAttribute {
	case UserIDExternalID:
		return firstNonEmpty(scimUserName, loginName), firstNonEmpty(scimExternalID, principalValue)
	default:
		return firstNonEmpty(scimUserName, principalValue, loginName), scimExternalID
	}
}

func firstValue(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
