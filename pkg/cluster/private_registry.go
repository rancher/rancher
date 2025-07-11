package cluster

import (
	"encoding/base64"
	"encoding/json"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/settings"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

type PrivateRegistry struct {
	// URL for the registry
	URL string `yaml:"url" json:"url,omitempty"`
}

// GetPrivateRegistryURL returns the URL of the private registry specified. It will return the cluster level registry if
// one is found, or the system default registry if no cluster level registry is found. If either is not found, it will
// return an empty string.
func GetPrivateRegistryURL(cluster *v3.Cluster) string {
	registry := GetPrivateRegistry(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

// GetPrivateRegistry returns a PrivateRegistry entry (or nil if one is not found) for the given
// clusters.management.cattle.io/v3 object. If a cluster-level registry is not defined, it will return the system
// default registry if one exists.
func GetPrivateRegistry(cluster *v3.Cluster) *PrivateRegistry {
	privateClusterLevelRegistry := GetPrivateClusterLevelRegistry(cluster)
	if privateClusterLevelRegistry != nil {
		return privateClusterLevelRegistry
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		return &PrivateRegistry{
			URL: settings.SystemDefaultRegistry.Get(),
		}
	}
	return nil
}

// GetPrivateClusterLevelRegistry returns the cluster-level registry for the given clusters.management.cattle.io/v3
// object (or nil if one is not found).
func GetPrivateClusterLevelRegistry(cluster *v3.Cluster) *PrivateRegistry {
	if cluster != nil && cluster.Spec.ImportedConfig != nil && cluster.Spec.ImportedConfig.PrivateRegistryURL != "" {
		return &PrivateRegistry{
			URL: cluster.Spec.ImportedConfig.PrivateRegistryURL,
		}
	}
	return nil
}

// GeneratePrivateRegistryEncodedDockerConfig generates a base64 encoded docker config JSON blob for the provided
// registry, and returns the registry url, the json credentials, and an error if one was encountered. If the cluster is
// nil or no registry is configured for a v2prov cluster, no registry url or json blob are returned, but there
// is no error returned, since not having a registry is not an error. If a registry is configured for the cluster such
// that we know what the URL is, but we do not have enough information to generate the auth config, we return the url,
// an empty string for the auth config, and no error, as we have determined where the private registry is, but the lack
// of secrets indicate to us that the registry does not need authentication to communicate.For v2prov clusters, we extract
// the username and password from the secret, and transform it into the expected docker config JSON format. This
// function should not be called with unmigrated clusters, although it is benign to call this function with assembler
// clusters, as the function will reassemble them anyway.
func GeneratePrivateRegistryEncodedDockerConfig(cluster *v3.Cluster, secretLister v1.SecretLister) (string, string, error) {
	var err error
	// Declare here so we don't need to check if the rkeClusterRegistryOrGlobalSystemDefault exists while working with v2prov
	var globalSystemDefaultURL string

	if cluster == nil {
		return "", "", nil
	}

	if globalSystemDefaultRegistry := GetPrivateRegistry(cluster); globalSystemDefaultRegistry != nil {
		globalSystemDefaultURL = globalSystemDefaultRegistry.URL

		// Return the private registry URL for imported clusters if it's configured. Skip generating
		// .dockerconfigjson since authentication is handled at the distro level for imported clusters.
		if cluster.Spec.ImportedConfig != nil && cluster.Spec.ImportedConfig.PrivateRegistryURL != "" {
			return globalSystemDefaultURL, "", nil
		}
	}

	// cluster.GetSecret("PrivateRegistryURL") will be empty if the cluster is
	// imported, or RKE2 with no cluster level registry configured.
	// For RKE2 with a cluster level registry configured, this is the
	// only reference to the registry URL available on the v3.Cluster.
	// Without it, we cannot generate the registry credentials (.dockerconfigjson)
	v2ProvRegistryURL := cluster.GetSecret(v3.ClusterPrivateRegistryURL)
	// if we don't get a v2ProvRegistryURL we can just return the image set on v1 Prov or the global system default one.
	if v2ProvRegistryURL == "" {
		return globalSystemDefaultURL, "", nil
	}

	// The PrivateRegistrySecret have the same name both for v1 or v2 provisioning clusters despite having different structures
	registrySecretName := cluster.GetSecret(v3.ClusterPrivateRegistrySecret)

	// If we reach this point we know that we have a registry URL set on the v2prov downstream cluster.
	// If it is a v2prov cluster without a registry URL the function would have already returned.
	// This last check is to see if the registry requires an authorization, if it doesn't we just return the v2ProvRegistryURL.
	if registrySecretName == "" {
		return v2ProvRegistryURL, "", nil
	}

	// If we have a registrySecretName (registry requires authentication) and this function reached this point
	// it is a v2 prov cluster. We need to decode that information to return it.
	registrySecret, err := secretLister.Get(cluster.Spec.FleetWorkspaceName, registrySecretName)
	if err != nil {
		return v2ProvRegistryURL, "", err
	}

	username := string(registrySecret.Data["username"])
	password := string(registrySecret.Data["password"])
	authConfig := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			v2ProvRegistryURL: credentialprovider.DockerConfigEntry{
				Username: username,
				Password: password,
			},
		},
	}

	registryJSON, err := json.Marshal(authConfig)
	if err != nil {
		return v2ProvRegistryURL, "", err
	}

	return v2ProvRegistryURL, base64.StdEncoding.EncodeToString(registryJSON), nil
}
