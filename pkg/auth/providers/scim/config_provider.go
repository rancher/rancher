package scim

import (
	"fmt"
	"strconv"

	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	configMapNamePrefix = "scim-config-"

	// UserIDUserName selects SCIM userName as the user principal identifier.
	UserIDUserName = "userName"
	// UserIDExternalID selects SCIM externalId as the user principal identifier.
	UserIDExternalID = "externalId"

	// GroupIDDisplayName selects SCIM displayName as the group principal identifier.
	GroupIDDisplayName = "displayName"
	// GroupIDExternalID selects SCIM externalId as the group principal identifier.
	GroupIDExternalID = "externalId"
)

// providerConfig holds SCIM provisioning settings for a single auth provider.
// Stored as a ConfigMap in cattle-global-data with name "scim-config-{provider}".
//
// Each field maps directly to a key in ConfigMap.Data:
//
//	enabled:          "true" | "false"   (default: false)
//	paused:           "true" | "false"   (default: false)
//	userIdAttribute:  "userName" | "externalId"   (default: "userName")
//	groupIdAttribute: "displayName" | "externalId" (default: "displayName")
type providerConfig struct {
	// Enabled controls whether SCIM provisioning is active for this provider.
	// The SCIM feature flag must also be enabled; this flag alone is not sufficient.
	Enabled bool

	// Paused suspends SCIM provisioning without revoking tokens or losing configuration.
	// When paused, the SCIM server returns 503 Service Unavailable.
	// Once unpaused, the IdP can re-sync and reconcile any changes that occurred in the interim.
	Paused bool

	// UserIDAttribute specifies which SCIM user attribute to use as the
	// identifier in the Rancher principal ID (e.g. "okta_user://<value>").
	// Must match what the auth provider's login flow uses.
	//
	// Supported values: "userName" (default), "externalId".
	UserIDAttribute string

	// GroupIDAttribute specifies which SCIM group attribute to use as the
	// identifier in the Rancher group principal ID.
	//
	// Supported values: "displayName" (default), "externalId".
	GroupIDAttribute string
}

func (c *providerConfig) userID(user scimUser) string {
	switch c.UserIDAttribute {
	case UserIDExternalID:
		return user.ExternalID
	default:
		return user.UserName
	}
}

func (c *providerConfig) groupID(displayName, externalID string) string {
	switch c.GroupIDAttribute {
	case GroupIDExternalID:
		return externalID
	default:
		return displayName
	}
}

func defaultProviderConfig() providerConfig {
	return providerConfig{
		UserIDAttribute:  UserIDUserName,
		GroupIDAttribute: GroupIDDisplayName,
	}
}

var validUserIDAttributes = map[string]bool{
	UserIDUserName:   true,
	UserIDExternalID: true,
}

var validGroupIDAttributes = map[string]bool{
	GroupIDDisplayName: true,
	GroupIDExternalID:  true,
}

// getProviderConfig loads the SCIM configuration for a provider from the ConfigMap.
// Returns default config if no ConfigMap exists.
func getProviderConfig(configMapCache wcorev1.ConfigMapCache, provider string) providerConfig {
	cfg := defaultProviderConfig()

	name := configMapNamePrefix + provider
	cm, err := configMapCache.Get(tokenSecretNamespace, name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("scim::getProviderConfig: failed to get configmap %s: %s", name, err)
		}
		return cfg
	}

	if v, ok := cm.Data["enabled"]; ok {
		cfg.Enabled, _ = strconv.ParseBool(v)
	}

	if v, ok := cm.Data["paused"]; ok {
		cfg.Paused, _ = strconv.ParseBool(v)
	}

	if v := cm.Data["userIdAttribute"]; v != "" {
		if validUserIDAttributes[v] {
			cfg.UserIDAttribute = v
		} else {
			logrus.Errorf("scim::getProviderConfig: invalid userIdAttribute %q in configmap %s, using default", v, name)
		}
	}

	if v := cm.Data["groupIdAttribute"]; v != "" {
		if validGroupIDAttributes[v] {
			cfg.GroupIDAttribute = v
		} else {
			logrus.Errorf("scim::getProviderConfig: invalid groupIdAttribute %q in configmap %s, using default", v, name)
		}
	}

	return cfg
}

func userPrincipalName(provider, id string) string {
	return fmt.Sprintf("%s_user://%s", provider, id)
}

func groupPrincipalName(provider, id string) string {
	return fmt.Sprintf("%s_group://%s", provider, id)
}
