package cluster

import (
	"encoding/base64"
	"encoding/json"

	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

func GetPrivateRepoURL(cluster *v3.Cluster) string {
	registry := GetPrivateRepo(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

func GetPrivateRepo(cluster *v3.Cluster) *v3.PrivateRegistry {
	if cluster != nil && cluster.Spec.RancherKubernetesEngineConfig != nil && len(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries) > 0 {
		config := cluster.Spec.RancherKubernetesEngineConfig
		return &config.PrivateRegistries[0]
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		return &v3.PrivateRegistry{
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

// This method serializes a ~/.docker/config.json file
func GeneratePrivateRegistryDockerConfig(privateRegistry *v3.PrivateRegistry) (string, error) {
	if privateRegistry == nil || privateRegistry.User == "" || privateRegistry.Password == "" {
		return "", nil
	}
	auth := credentialprovider.DockerConfigEntry{
		Username: privateRegistry.User,
		Password: privateRegistry.Password,
	}

	config := credentialprovider.DockerConfigJson{
		Auths: map[string]credentialprovider.DockerConfigEntry{privateRegistry.URL: auth},
	}

	bt, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bt), nil
}
