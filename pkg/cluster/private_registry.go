package cluster

import (
	"encoding/base64"
	"encoding/json"

	"github.com/docker/docker/api/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
)

func GetPrivateRepoURL(cluster *v3.Cluster) string {
	registry := GetPrivateRepo(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

func GetPrivateRepo(cluster *v3.Cluster) *rketypes.PrivateRegistry {
	if cluster != nil && cluster.Spec.RancherKubernetesEngineConfig != nil && len(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries) > 0 {
		config := cluster.Spec.RancherKubernetesEngineConfig
		return &config.PrivateRegistries[0]
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		return &rketypes.PrivateRegistry{
			URL: settings.SystemDefaultRegistry.Get(),
		}
	}
	return nil
}

func GenerateClusterPrivateRegistryDockerConfig(cluster *v3.Cluster) (string, error) {
	if cluster == nil {
		return "", nil
	}
	return GeneratePrivateRegistryDockerConfig(GetPrivateRepo(cluster))
}

// This method generates base64 encoded credentials for the registry
func GeneratePrivateRegistryDockerConfig(privateRegistry *rketypes.PrivateRegistry) (string, error) {
	if privateRegistry == nil || privateRegistry.User == "" || privateRegistry.Password == "" {
		return "", nil
	}
	authConfig := types.AuthConfig{
		Username: privateRegistry.User,
		Password: privateRegistry.Password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}
