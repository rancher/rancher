package cluster

import (
	"encoding/base64"
	"encoding/json"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

// GetPrivateRepoURL returns the URL of the private repo specified. It will return the cluster level repo if one is
// found, or the system default registry if no cluster level registry is found. If either is not found, it will return
// an empty string.
func GetPrivateRepoURL(cluster *v3.Cluster) string {
	registry := GetPrivateRepo(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

// GetPrivateRepo returns a PrivateRegistry entry (or nil if one is not found) for the given
// clusters.management.cattle.io/v3 object. If a cluster-level registry is not defined, it will return the system
// default registry if one exists.
func GetPrivateRepo(cluster *v3.Cluster) *rketypes.PrivateRegistry {
	privateClusterLevelRepo := GetPrivateClusterLevelRepo(cluster)
	if privateClusterLevelRepo != nil {
		return privateClusterLevelRepo
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		return &rketypes.PrivateRegistry{
			URL: settings.SystemDefaultRegistry.Get(),
		}
	}
	return nil
}

// GetPrivateClusterLevelRepo returns the cluster-level repo for the given clusters.management.cattle.io/v3 object (or nil if one is not found).
func GetPrivateClusterLevelRepo(cluster *v3.Cluster) *rketypes.PrivateRegistry {
	if cluster != nil && cluster.Spec.RancherKubernetesEngineConfig != nil && len(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries) > 0 {
		config := cluster.Spec.RancherKubernetesEngineConfig
		return &config.PrivateRegistries[0]
	}
	return nil
}

// TransformProvV2RegistryCredentialsToDockerConfigJSON transforms a ProvV2 registry secret into a credentialprovider.DockerConfigJSON secret
func TransformProvV2RegistryCredentialsToDockerConfigJSON(url string, registryCredentials *corev1.Secret) (map[string][]byte, error) {
	username := string(registryCredentials.Data["username"])
	password := string(registryCredentials.Data["password"])
	authConfig := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			url: credentialprovider.DockerConfigEntry{
				Username: username,
				Password: password,
			},
		},
	}
	j, err := json.Marshal(authConfig)
	return map[string][]byte{".dockerconfigjson": j}, err
}

func GenerateClusterPrivateRegistryDockerConfig(cluster *v3.Cluster) (string, error) {
	if cluster == nil {
		return "", nil
	}
	return GeneratePrivateRegistryDockerConfig(GetPrivateRepo(cluster), nil)
}

// GeneratePrivateRegistryDockerConfig generates base64 encoded credentials for the provided registry
func GeneratePrivateRegistryDockerConfig(privateRegistry *rketypes.PrivateRegistry, registrySecret []byte) (string, error) {
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
		if len(registrySecret) > 0 {
			err := json.Unmarshal(registrySecret, &dockerCfg) // check to see if registrySecret is in the correct format
			if err != nil {
				logrus.Debug("Failed to parse dockerconfig for registry secret: " + err.Error())
				return "", err
			}
			if _, ok := dockerCfg.Auths[privateRegistry.URL]; ok {
				return base64.URLEncoding.EncodeToString(registrySecret), nil
			}
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
