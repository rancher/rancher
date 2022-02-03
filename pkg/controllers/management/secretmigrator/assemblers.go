package secretmigrator

import (
	"encoding/json"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

// AssemblePrivateRegistryCredential looks up the registry Secret and inserts the keys into the PrivateRegistries list on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssemblePrivateRegistryCredential(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if cluster.Spec.RancherKubernetesEngineConfig == nil || len(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries) == 0 {
		return spec, nil
	}
	if cluster.Status.PrivateRegistrySecret == "" {
		if cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries[0].Password != "" {
			logrus.Warnf("[secretmigrator] secrets for cluster %s are not finished migrating", cluster.Name)
		}
		return spec, nil

	}
	registrySecret, err := secretLister.Get(secretNamespace, cluster.Status.PrivateRegistrySecret)
	if err != nil {
		return spec, err
	}
	dockerCfg := credentialprovider.DockerConfigJSON{}
	err = json.Unmarshal(registrySecret.Data[".dockerconfigjson"], &dockerCfg)
	if err != nil {
		return spec, err
	}
	for i, privateRegistry := range cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries {
		if reg, ok := dockerCfg.Auths[privateRegistry.URL]; ok {
			spec.RancherKubernetesEngineConfig.PrivateRegistries[i].User = reg.Username
			spec.RancherKubernetesEngineConfig.PrivateRegistries[i].Password = reg.Password
		}
	}
	return spec, nil
}
