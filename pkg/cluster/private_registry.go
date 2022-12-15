package cluster

import (
	"encoding/base64"
	"encoding/json"

	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

// GetPrivateRegistryURL returns the URL of the private registry specified. It will return the cluster level repo if
// one is found, or the system default registry if no cluster level registry is found. If either is not found, it will
// return an empty string.
func GetPrivateRegistryURL(rkeConfig *rketypes.RancherKubernetesEngineConfig) string {
	registry := GetPrivateRegistry(rkeConfig)
	if registry == nil {
		return ""
	}
	return registry.URL
}

// GetPrivateRegistry returns a PrivateRegistry entry (or nil if one is not found) for the given
// clusters.management.cattle.io/v3 object. If a cluster-level registry is not defined, it will return the system
// default registry if one exists.
func GetPrivateRegistry(rkeConfig *rketypes.RancherKubernetesEngineConfig) *rketypes.PrivateRegistry {
	if rkeConfig != nil && len(rkeConfig.PrivateRegistries) > 0 {
		return &rkeConfig.PrivateRegistries[0]
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		return &rketypes.PrivateRegistry{
			URL: settings.SystemDefaultRegistry.Get(),
		}
	}
	return nil
}

func GenerateClusterPrivateRegistryDockerConfig(rkeConfig *rketypes.RancherKubernetesEngineConfig) (string, error) {
	if rkeConfig == nil {
		return "", nil
	}

	return GeneratePrivateRegistryDockerConfig(GetPrivateRegistry(rkeConfig))
}

// GeneratePrivateRegistryDockerConfig method generates base64 encoded credentials for the registry.
// This function assumes that the private registry secrets have been assembled prior to calling.
// It does not modify the privateRegistry.
func GeneratePrivateRegistryDockerConfig(privateRegistry *rketypes.PrivateRegistry) (string, error) {
	if privateRegistry == nil {
		return "", nil
	}

	if privateRegistry.ECRCredentialPlugin != nil {
		// generate ecr authConfig
		authConfig, err := util.ECRCredentialPlugin(privateRegistry.ECRCredentialPlugin, privateRegistry.URL)
		if err != nil {
			return "", err
		}
		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			return "", err
		}
		return base64.URLEncoding.EncodeToString(encodedJSON), nil
	}
	if privateRegistry.User == "" || privateRegistry.Password == "" {
		return "", nil
	}
	authConfig := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			privateRegistry.URL: credentialprovider.DockerConfigEntry{
				Username: privateRegistry.User,
				Password: privateRegistry.Password,
			},
		},
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}
