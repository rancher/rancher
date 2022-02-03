package cluster

import (
	"encoding/base64"
	"encoding/json"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
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
	return GeneratePrivateRegistryDockerConfig(GetPrivateRepo(cluster), nil)
}

// This method generates base64 encoded credentials for the registry
func GeneratePrivateRegistryDockerConfig(privateRegistry *rketypes.PrivateRegistry, registrySecret *corev1.Secret) (string, error) {
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
	if registrySecret != nil {
		privateRegistry = privateRegistry.DeepCopy()
		dockerCfg := credentialprovider.DockerConfigJSON{}
		err := json.Unmarshal(registrySecret.Data[".dockerconfigjson"], &dockerCfg)
		if err != nil {
			return "", err
		}
		if reg, ok := dockerCfg.Auths[privateRegistry.URL]; ok {
			privateRegistry.User = reg.Username
			privateRegistry.Password = reg.Password
		}
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
