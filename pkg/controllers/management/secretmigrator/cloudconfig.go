package secretmigrator

import (
	"strings"

	"github.com/rancher/norman/types/convert"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// cloudConfigSecretRemover deletes any detected secrets within the clusters cloud-provider-config
// field which have the expected secret format, the AuthorizedSecretAnnotation,
// and the AuthorizedSecretDeletesOnClusterRemovalAnnotation. Secrets without the AuthorizedSecretDeletesOnClusterRemovalAnnotation
// annotation are not removed, and are assumed to be user created. Only secrets without any OwnerReferences are removed.
func (h *handler) cloudConfigSecretRemover(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil || cluster.Spec.RKEConfig == nil || cluster.Name == "local" {
		return cluster, nil
	}

	for _, e := range cluster.Spec.RKEConfig.MachineSelectorConfig {
		cloudProviderConfig := convert.ToString(e.Config.Data["cloud-provider-config"])

		// check if the cloud-provider-config value points to a secret
		if cloudProviderConfig == "" || !strings.HasPrefix(cloudProviderConfig, "secret://") {
			continue
		}

		cleanSecretNamespaceAndName := strings.TrimPrefix(cloudProviderConfig, "secret://")
		namespaceAndName := strings.Split(cleanSecretNamespaceAndName, ":")

		// ensure the secret format is proper
		if len(namespaceAndName) != 2 {
			logrus.Errorf("[cloud-config-secret-remover] error encountered while handling secrets deletion for cloud-provider-config: provided secret value is not of form secret://namespace:name")
			continue
		}

		secret, err := h.migrator.secrets.GetNamespaced(namespaceAndName[0], namespaceAndName[1], metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("[cloud-config-secret-remover] error encountered while retrieving secret %s:%s defined within cloud-provider-config: %s", namespaceAndName[0], namespaceAndName[1], err)
			continue
		}

		authorizedForCluster := secret.Annotations[AuthorizedSecretAnnotation]
		deleteSecretOnClusterRemoval := secret.Annotations[AuthorizedSecretDeletesOnClusterRemovalAnnotation]

		if authorizedForCluster == cluster.Name && deleteSecretOnClusterRemoval == "true" {
			if len(secret.OwnerReferences) == 0 {
				h.migrator.CleanupKnownSecrets([]*corev1.Secret{secret})
			}
		}
	}

	return cluster, nil
}
