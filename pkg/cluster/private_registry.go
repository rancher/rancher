package cluster

import (
	"encoding/base64"
	"encoding/json"

	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

func GetPrivateRegistryURL(rkeConfig *rketypes.RancherKubernetesEngineConfig) string {
	registry := GetPrivateRegistry(rkeConfig)
	if registry == nil {
		return ""
	}
	return registry.URL
}

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
