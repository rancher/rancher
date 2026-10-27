package provider

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// configNameClaim is the access and id_token claim that holds the name of the
// AuthConfig that issued the Rancher token, so that the userinfo endpoint can
// return the same groups as the token endpoint. It replaces the deprecated
// auth_provider claim, which holds the provider type.
const configNameClaim = "config_name"

// groupsForConfig returns the names of the user's groups from the named
// AuthConfig. Groups from the user's other AuthConfigs are left out, as they
// weren't provided by the login that issued the token.
func groupsForConfig(attribs *v3.UserAttribute, configName string) []string {
	if attribs == nil {
		return nil
	}

	var groups []string
	for _, principal := range attribs.GroupPrincipals[configName].Items {
		groups = append(groups, principal.Name)
	}

	return groups
}
