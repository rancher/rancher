package secretmigrator

import (
	"encoding/json"
	"reflect"

	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	secretNamespace = namespace.GlobalNamespace
)

func (h *handler) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}
	if v3.ClusterConditionSecretsMigrated.IsTrue(cluster) {
		logrus.Tracef("[secretmigrator] cluster %s already migrated", cluster.Name)
		return cluster, nil
	}
	obj, err := v3.ClusterConditionSecretsMigrated.Do(cluster, func() (runtime.Object, error) {
		// privateRegistries
		if cluster.Status.PrivateRegistrySecret == "" {
			logrus.Tracef("[secretmigrator] migrating private registry secrets for cluster %s", cluster.Name)
			regSecret, err := h.migrator.CreateOrUpdatePrivateRegistrySecret(cluster.Status.PrivateRegistrySecret, cluster.Spec.RancherKubernetesEngineConfig, cluster)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for cluster %s, will retry: %v", cluster.Name, err)
				return nil, err
			}
			if regSecret != nil {
				logrus.Tracef("[secretmigrator] private registry secret found for cluster %s", cluster.Name)
				cluster.Status.PrivateRegistrySecret = regSecret.Name
				cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries)
				if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
					cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries)
				}
				if cluster.Status.FailedSpec != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig != nil {
					cluster.Status.FailedSpec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(cluster.Status.FailedSpec.RancherKubernetesEngineConfig.PrivateRegistries)
				}
				clusterCopy, err := h.clusters.Update(cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, regSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				cluster = clusterCopy
			}
		}

		logrus.Tracef("[secretmigrator] setting cluster condition and updating cluster %s", cluster.Name)
		v3.ClusterConditionSecretsMigrated.True(cluster)
		return h.clusters.Update(cluster)
	})
	return obj.(*v3.Cluster), err
}

// CreateOrUpdatePrivateRegistrySecret accepts an optional secret name and a RancherKubernetesEngineConfig object and creates a dockerconfigjson Secret
// containing the login credentials for every registry in the array, if there are any.
// If an owner is passed, the owner is set as an owner reference on the Secret. If no owner is passed,
// the caller is responsible for calling UpdateSecretOwnerReference once the owner is known.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdatePrivateRegistrySecret(secretName string, rkeConfig *v3.RancherKubernetesEngineConfig, owner *v3.Cluster) (*corev1.Secret, error) {
	if rkeConfig == nil {
		return nil, nil
	}
	rkeConfig = rkeConfig.DeepCopy()
	privateRegistries := rkeConfig.PrivateRegistries
	if len(privateRegistries) == 0 {
		return nil, nil
	}
	var existing *corev1.Secret
	if secretName != "" {
		var err error
		existing, err = m.secretLister.Get(secretNamespace, secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	existingRegistry := credentialprovider.DockerConfigJson{}
	active := make(map[string]struct{})
	needsUpdate := false
	registrySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:         secretName, // if empty, the secret will be created with a generated name
			GenerateName: "cluster-registry-",
			Namespace:    secretNamespace,
		},
		Data: map[string][]byte{},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	if owner != nil {
		registrySecret.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: owner.APIVersion,
				Kind:       owner.Kind,
				Name:       owner.Name,
				UID:        owner.UID,
			},
		}
	}
	for _, privateRegistry := range privateRegistries {
		active[privateRegistry.URL] = struct{}{}
		if privateRegistry.Password == "" {
			continue
		}
		registry := credentialprovider.DockerConfigJson{
			Auths: credentialprovider.DockerConfig{
				privateRegistry.URL: credentialprovider.DockerConfigEntry{
					Username: privateRegistry.User,
					Password: privateRegistry.Password,
				},
			},
		}
		registryJSON, err := json.Marshal(registry)
		if err != nil {
			return nil, err
		}
		registrySecret.Data = map[string][]byte{
			corev1.DockerConfigJsonKey: registryJSON,
		}
		if existing == nil {
			registrySecret, err = m.secrets.Create(registrySecret)
			if err != nil {
				return nil, err
			}
		} else if !reflect.DeepEqual(existing.Data, registrySecret.Data) {
			err = json.Unmarshal(existing.Data[corev1.DockerConfigJsonKey], &existingRegistry)
			if err != nil {
				return nil, err
			}
			// limitation: if a URL is repeated in the privateRegistries list, it will be overwritten in the registry secret
			existingRegistry.Auths[privateRegistry.URL] = registry.Auths[privateRegistry.URL]
			registrySecret.Data[corev1.DockerConfigJsonKey], err = json.Marshal(existingRegistry)
			if err != nil {
				return nil, err
			}
			needsUpdate = true
		}
	}
	if existing != nil {
		for url := range existingRegistry.Auths {
			if _, ok := active[url]; !ok {
				delete(existingRegistry.Auths, url)
				var err error
				registrySecret.Data[corev1.DockerConfigJsonKey], err = json.Marshal(existingRegistry)
				if err != nil {
					return nil, err
				}
				needsUpdate = true
			}
		}
	}
	if needsUpdate {
		return m.secrets.Update(registrySecret)
	}
	return registrySecret, nil
}

// CleanRegistries unsets the password of every private registry in the list.
// Must be called after passwords have been migrated.
func CleanRegistries(privateRegistries []v3.PrivateRegistry) []v3.PrivateRegistry {
	for i := range privateRegistries {
		privateRegistries[i].Password = ""
	}
	return privateRegistries
}

// UpdateSecretOwnerReference sets an object as owner of a given Secret and updates the Secret.
// The object must be a non-namespaced resource.
func (m *Migrator) UpdateSecretOwnerReference(secret *corev1.Secret, owner metav1.OwnerReference) error {
	secret.OwnerReferences = []metav1.OwnerReference{owner}
	_, err := m.secrets.Update(secret)
	return err
}
