package image

import (
	"path"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
)

func ResolveWithControlPlane(image string, cp *rkev1.RKEControlPlane) string {
	url, _ := GetPrivateRepoURLFromControlPlane(cp)
	return resolve(url, image)
}

func ResolveWithCluster(image string, cluster *v1.Cluster) string {
	url, _ := GetPrivateRepoURLFromCluster(cluster)
	return resolve(url, image)
}

func resolve(reg, image string) string {
	if reg != "" && !strings.HasPrefix(image, reg) {
		//Images from Dockerhub Library repo, we add rancher prefix when using private registry
		if !strings.Contains(image, "/") {
			image = "rancher/" + image
		}
		return path.Join(reg, image)
	}

	return image
}

// GetPrivateRepoSecretFromCluster returns the name of the secret containing the credentials for the cluster level system-default-registry.
func GetPrivateRepoSecretFromCluster(cluster *v1.Cluster) (string, bool) {
	url, isGlobalDefault := GetPrivateRepoURLFromCluster(cluster)
	if cluster != nil && cluster.Spec.RKEConfig != nil && cluster.Spec.RKEConfig.Registries != nil {
		config, ok := cluster.Spec.RKEConfig.Registries.Configs[url]
		if ok {
			return config.AuthConfigSecretName, false
		}
	}

	// fall back to the GSDR if configured, but only return the first pull secret. Due to the format of the containerd registry authentication file
	// only one credential can be configured per hostname.
	globalPullSecrets := settings.SystemDefaultRegistryPullSecrets.Get()
	if isGlobalDefault && globalPullSecrets != "" {
		for _, entry := range strings.Split(globalPullSecrets, ",") {
			secret := strings.TrimSpace(entry)
			if secret != "" {
				return secret, true
			}
		}
	}

	return "", false
}

func GetRegistryAuthSecretForHostname(cluster *v1.Cluster, url string) string {
	if cluster == nil || cluster.Spec.RKEConfig == nil || cluster.Spec.RKEConfig.Registries == nil {
		return ""
	}
	for regUrl, registry := range cluster.Spec.RKEConfig.Registries.Configs {
		if regUrl == url {
			return registry.AuthConfigSecretName
		}
	}
	return ""
}

// GetPrivateRepoURLFromCluster returns the system-default-registry URL from either the clusters
// machineGlobalConfig, or one of its machineSelectorConfig's which has no label selectors.
// If no cluster level system-default-registry is configured, it will return the global system-default-registry.
func GetPrivateRepoURLFromCluster(cluster *v1.Cluster) (string, bool) {
	if cluster != nil && cluster.Spec.RKEConfig != nil {
		return getPrivateRepoURL(cluster.Spec.RKEConfig.MachineGlobalConfig, cluster.Spec.RKEConfig.MachineSelectorConfig)
	}

	return settings.SystemDefaultRegistry.Get(), true
}

// GetPrivateRepoURLFromControlPlane returns the system-default-registry URL from either the control planes
// machineGlobalConfig, or one of its machineSelectorConfig's which has no label selectors.
// If no cluster level system-default-registry is configured, it will return the global system-default-registry and a boolean
// indicating that the value was pulled from the global configuration.
func GetPrivateRepoURLFromControlPlane(cp *rkev1.RKEControlPlane) (string, bool) {
	if cp != nil {
		return getPrivateRepoURL(cp.Spec.MachineGlobalConfig, cp.Spec.MachineSelectorConfig)
	}

	return settings.SystemDefaultRegistry.Get(), true
}

// getPrivateRepoURL retrieves the configured system-default-registry URL from either the machineGlobalConfig or machineSelectorConfig (in that order),
// and returns the URL along with a boolean indicating whether the returned URL is from the global configuration.
func getPrivateRepoURL(machineGlobalConfig rkev1.GenericMap, machineSelectorConfig []rkev1.RKESystemConfig) (string, bool) {
	for key, val := range machineGlobalConfig.Data {
		if val, ok := val.(string); ok && key == "system-default-registry" {
			return val, false
		}
	}

	for _, config := range machineSelectorConfig {
		if registryVal, ok := config.Config.Data["system-default-registry"]; config.MachineLabelSelector == nil && ok {
			if registry, ok := registryVal.(string); ok {
				return registry, false
			}
		}
	}

	return settings.SystemDefaultRegistry.Get(), true
}

func GetDesiredAgentImage(cp *rkev1.RKEControlPlane, mgmtCluster *v3.Cluster) string {
	desiredAgent := mgmtCluster.Spec.DesiredAgentImage
	if mgmtCluster.Spec.AgentImageOverride != "" {
		desiredAgent = mgmtCluster.Spec.AgentImageOverride
	}
	if desiredAgent == "" || desiredAgent == "fixed" {
		desiredAgent = ResolveWithControlPlane(settings.AgentImage.Get(), cp)
	}
	return desiredAgent
}

func GetDesiredAuthImage(cp *rkev1.RKEControlPlane, mgmtCluster *v3.Cluster) string {
	var desiredAuth string
	if mgmtCluster.Spec.LocalClusterAuthEndpoint.Enabled {
		desiredAuth = mgmtCluster.Spec.DesiredAuthImage
		if desiredAuth == "" || desiredAuth == "fixed" {
			desiredAuth = ResolveWithControlPlane(settings.AuthImage.Get(), cp)
		}
	}
	return desiredAuth
}
