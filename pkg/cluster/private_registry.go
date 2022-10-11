package cluster

import (
	"encoding/base64"
	"encoding/json"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

func GetPrivateRepoURL(cluster *apimgmtv3.Cluster) string {
	registry := GetPrivateRepo(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

func GetPrivateRepo(cluster *apimgmtv3.Cluster) *rketypes.PrivateRegistry {
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

func GenerateClusterPrivateRegistryDockerConfig(cluster *apimgmtv3.Cluster) (string, error) {
	if cluster == nil {
		return "", nil
	}

	return GeneratePrivateRegistryDockerConfig(GetPrivateRepo(cluster))
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
