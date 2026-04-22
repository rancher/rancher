package scim

import (
	"encoding/json"
	"fmt"

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
type providerConfig struct {
	// UserIDAttribute specifies which SCIM user attribute to use as the
	// identifier in the Rancher principal ID (e.g. "okta_user://<value>").
	// Must match what the auth provider's login flow uses.
	//
	// Supported values: "userName" (default), "externalId".
	UserIDAttribute string `json:"userIdAttribute,omitempty"`

	// GroupIDAttribute specifies which SCIM group attribute to use as the
	// identifier in the Rancher group principal ID.
	//
	// Supported values: "displayName" (default), "externalId".
	GroupIDAttribute string `json:"groupIdAttribute,omitempty"`
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

	data, ok := cm.Data["config"]
	if !ok {
		return cfg
	}

	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		logrus.Errorf("scim::getProviderConfig: failed to parse configmap %s: %s", name, err)
		return defaultProviderConfig()
	}

	if cfg.UserIDAttribute == "" {
		cfg.UserIDAttribute = UserIDUserName
	}
	if cfg.GroupIDAttribute == "" {
		cfg.GroupIDAttribute = GroupIDDisplayName
	}

	if !validUserIDAttributes[cfg.UserIDAttribute] {
		logrus.Errorf("scim::getProviderConfig: invalid userIdAttribute %q in configmap %s, using default", cfg.UserIDAttribute, name)
		cfg.UserIDAttribute = UserIDUserName
	}
	if !validGroupIDAttributes[cfg.GroupIDAttribute] {
		logrus.Errorf("scim::getProviderConfig: invalid groupIdAttribute %q in configmap %s, using default", cfg.GroupIDAttribute, name)
		cfg.GroupIDAttribute = GroupIDDisplayName
	}

	return cfg
}

func userPrincipalName(provider, id string) string {
	return fmt.Sprintf("%s_user://%s", provider, id)
}

func groupPrincipalName(provider, id string) string {
	return fmt.Sprintf("%s_group://%s", provider, id)
}
