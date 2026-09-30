// Package scimconfig reads the per-provider SCIM configuration. It imports no
// auth code, so the SCIM handlers, the user manager and the provider refresher
// can all use it.
package scimconfig

import (
	"strconv"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Namespace is the namespace of the SCIM configuration ConfigMaps.
const Namespace = namespace.GlobalNamespace

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

// Config holds SCIM provisioning settings for a single auth provider.
// Stored as a ConfigMap in cattle-global-data with name "scim-config-{provider}".
//
// Each field maps directly to a key in ConfigMap.Data:
//
//	enabled:                    "true" | "false"                (default: false)
//	paused:                     "true" | "false"                (default: false)
//	userIdAttribute:            "userName" | "externalId"       (default: "userName")
//	groupIdAttribute:           "displayName" | "externalId"    (default: "displayName")
//	rateLimitRequestsPerSecond: integer                         (default: 0 = disabled)
//	rateLimitBurst:             integer                         (default: 10)
type Config struct {
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

	// RateLimitRequestsPerSecond caps SCIM API requests per second for this provider.
	// 0 disables rate limiting. In HA setups each replica enforces its own limit,
	// so the effective cluster-wide rate is roughly this value multiplied by the number of replicas.
	RateLimitRequestsPerSecond int

	// RateLimitBurst is how many requests this provider can make in a quick burst
	// before the steady-state rate kicks in.
	RateLimitBurst int
}

// UserID returns the SCIM user attribute the user principal ID is built from.
func (c Config) UserID(userName, externalID string) string {
	switch c.UserIDAttribute {
	case UserIDExternalID:
		return externalID
	default:
		return userName
	}
}

// GroupID returns the SCIM group attribute the group principal ID is built from.
func (c Config) GroupID(displayName, externalID string) string {
	switch c.GroupIDAttribute {
	case GroupIDExternalID:
		return externalID
	default:
		return displayName
	}
}

const defaultRateLimitBurst = 10

// Default returns the configuration used when a provider has no ConfigMap.
func Default() Config {
	return Config{
		UserIDAttribute:  UserIDUserName,
		GroupIDAttribute: GroupIDDisplayName,
		RateLimitBurst:   defaultRateLimitBurst,
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

// Get loads the SCIM configuration for a provider from its ConfigMap.
// Returns Default() if no ConfigMap exists.
func Get(configMapCache wcorev1.ConfigMapCache, provider string) Config {
	cfg := Default()

	name := configMapNamePrefix + provider
	cm, err := configMapCache.Get(Namespace, name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("scimconfig::Get: failed to get configmap %s: %s", name, err)
		}
		return cfg
	}

	if v, ok := cm.Data["enabled"]; ok {
		enabled, err := strconv.ParseBool(v)
		if err != nil {
			logrus.Errorf("scimconfig::Get: invalid enabled value %q in configmap %s, using default", v, name)
		} else {
			cfg.Enabled = enabled
		}
	}

	if v, ok := cm.Data["paused"]; ok {
		paused, err := strconv.ParseBool(v)
		if err != nil {
			logrus.Errorf("scimconfig::Get: invalid paused value %q in configmap %s, using default", v, name)
		} else {
			cfg.Paused = paused
		}
	}

	if v := cm.Data["userIdAttribute"]; v != "" {
		if validUserIDAttributes[v] {
			cfg.UserIDAttribute = v
		} else {
			logrus.Errorf("scimconfig::Get: invalid userIdAttribute %q in configmap %s, using default", v, name)
		}
	}

	if v := cm.Data["groupIdAttribute"]; v != "" {
		if validGroupIDAttributes[v] {
			cfg.GroupIDAttribute = v
		} else {
			logrus.Errorf("scimconfig::Get: invalid groupIdAttribute %q in configmap %s, using default", v, name)
		}
	}

	if v, ok := cm.Data["rateLimitRequestsPerSecond"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			logrus.Errorf("scimconfig::Get: invalid rateLimitRequestsPerSecond value %q in configmap %s, using default", v, name)
		} else {
			cfg.RateLimitRequestsPerSecond = n
		}
	}

	if v, ok := cm.Data["rateLimitBurst"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			logrus.Errorf("scimconfig::Get: invalid rateLimitBurst value %q in configmap %s, using default", v, name)
		} else {
			cfg.RateLimitBurst = n
		}
	}

	return cfg
}

// Enabled reports whether SCIM is enabled for provider: the scim feature flag
// is on and the provider's ConfigMap has enabled set to true. A paused
// provider counts as enabled.
func Enabled(configMapCache wcorev1.ConfigMapCache, provider string) bool {
	return features.SCIM.Enabled() && Get(configMapCache, provider).Enabled
}
