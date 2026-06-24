package autoscaler

import (
	"errors"
	"reflect"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// manageHelmOpSecrets determines if the incoming cluster should use the global-default or cluster-scoped
// credentials when deploying the autoscaler HelmOp, and returns the names of the Helm secret and image pull
// secret to pass to the HelmOp bundle.
func (h *autoscalerHandler) manageHelmOpSecrets(capiCluster *capi.Cluster) (helmOpSecretName string, imagePullSecretName string, err error) {
	provCluster, err := h.clusterCache.Get(capiCluster.Namespace, capiCluster.Name)
	if err != nil {
		return "", "", err
	}

	// A cluster may have one or more Registries.Configs entries for containerd purposes
	// (TLS, mirrors), but does not cover the registry used for the autoscaler chart/image.
	// In this case, the global credentials should be used. When the cluster _does_ have valid
	// credentials for the chart host, the cluster-scoped secrets are used.
	if image.GetRegistryAuthSecretForHostname(provCluster, autoScalerChartRepositoryHost()) != "" {
		username, password, err := h.findClusterLevelAutoScalerHostnameCreds(provCluster)
		if err != nil && !errors.Is(err, cluster.ErrRegistryHostnameNotFound) {
			return "", "", err
		}
		if username != "" && password != "" {
			helmOpSecretName, err = h.ensureClusterScopedHelmOpSecretInNamespace(provCluster, capiCluster, username, password)
			if err != nil {
				return "", "", err
			}
			imagePullSecretName, err = h.ensureClusterScopedImagePullSecretInNamespace(provCluster, capiCluster, username, password)
			if err != nil {
				return "", "", err
			}
			return helmOpSecretName, imagePullSecretName, nil
		}
	}

	if err := h.cleanupClusterScopedSecrets(provCluster, capiCluster); err != nil {
		return "", "", err
	}

	return h.ensureRootHelmOpSecrets()
}

// ensureRootHelmOpSecrets manages the shared Helm basic-auth secret and dockerconfigjson image
// pull secret in the fleet-default namespace. Returns the names of the Helm secret and image
// pull secret, or empty strings if no credentials are available (in which case any previously
// created secrets are also deleted). Creating two shared root secrets helps reduce the number
// of objects created when there are many clusters using the GSDR and the autoscaler.
func (h *autoscalerHandler) ensureRootHelmOpSecrets() (string, string, error) {
	username, password, err := h.findGlobalClusterAutoScalerHostnameCreds()
	if err != nil {
		return "", "", err
	}

	if username == "" || password == "" {
		if err := h.deleteSecretIfExists("fleet-default", autoscalerHelmSecretResourceName); err != nil {
			return "", "", err
		}
		if err := h.deleteSecretIfExists("fleet-default", autoscalerChartImagePullSecretName); err != nil {
			return "", "", err
		}
		return "", "", nil
	}

	helmOpSecret, err := h.upsertSecret("fleet-default", autoscalerHelmSecretResourceName, "", basicAuthSecretData(username, password), nil)
	if err != nil {
		return "", "", err
	}

	dockerConfigData, err := dockerConfigSecretData(autoScalerChartRepositoryHost(), username, password)
	if err != nil {
		return "", "", err
	}

	pullSecret, err := h.upsertSecret("fleet-default", autoscalerChartImagePullSecretName, v1.SecretTypeDockerConfigJson, dockerConfigData, nil)
	if err != nil {
		return "", "", err
	}

	return helmOpSecret.Name, pullSecret.Name, nil
}

// cleanupClusterScopedSecrets removes the cluster-scoped Helm secret and image pull secret
// that may have been created when the cluster previously used a per-cluster registry configuration.
// This is called when a cluster transitions back to the global-default registry, to avoid
// leaving orphaned secrets behind in the cluster's namespace.
func (h *autoscalerHandler) cleanupClusterScopedSecrets(provCluster *provv1.Cluster, capiCluster *capi.Cluster) error {
	if err := h.deleteSecretIfExists(capiCluster.Namespace, helmOpSecretName(capiCluster.Name)); err != nil {
		return err
	}
	return h.deleteSecretIfExists(provCluster.Namespace, autoscalerClusterScopedImagePullSecretName(provCluster.Name))
}

// ensureClusterScopedHelmOpSecretInNamespace creates or updates a basic-auth Helm secret in the
// CAPI cluster's namespace using pre-fetched cluster-level autoscaler chart credentials.
// Credentials are resolved once in manageHelmOpSecrets to avoid redundant secret cache reads.
func (h *autoscalerHandler) ensureClusterScopedHelmOpSecretInNamespace(provCluster *provv1.Cluster, capiCluster *capi.Cluster, username, password string) (string, error) {
	s, err := h.upsertSecret(capiCluster.Namespace, helmOpSecretName(capiCluster.Name), "", basicAuthSecretData(username, password), ownerReference(capiCluster))
	if err != nil {
		return "", err
	}
	return s.Name, nil
}

// ensureClusterScopedImagePullSecretInNamespace creates or updates a dockerconfigjson image pull
// secret in the provisioning cluster's namespace for pulling the autoscaler chart image.
// If the chart host matches the cluster's system default registry, no secret is created because
// credentials are already configured at provisioning time.
func (h *autoscalerHandler) ensureClusterScopedImagePullSecretInNamespace(provCluster *provv1.Cluster, capiCluster *capi.Cluster, username, password string) (string, error) {
	chartHost := autoScalerChartRepositoryHost()
	sdrURL, _ := image.GetPrivateRepoURLFromCluster(provCluster)
	if chartHost == sdrURL {
		return "", nil
	}

	dockerConfigData, err := dockerConfigSecretData(chartHost, username, password)
	if err != nil {
		return "", err
	}

	s, err := h.upsertSecret(provCluster.Namespace, autoscalerClusterScopedImagePullSecretName(provCluster.Name), v1.SecretTypeDockerConfigJson, dockerConfigData, ownerReference(capiCluster))
	if err != nil {
		return "", err
	}
	return s.Name, nil
}

// basicAuthSecretData returns a secret data map containing username and password fields,
// suitable for use as a Helm basic-auth credential secret.
func basicAuthSecretData(username, password string) map[string][]byte {
	return map[string][]byte{
		"username": []byte(username),
		"password": []byte(password),
	}
}

// dockerConfigSecretData builds a dockerconfigjson data map for the given registry hostname
// and credentials.
func dockerConfigSecretData(host, username, password string) (map[string][]byte, error) {
	cfg, err := cluster.BuildDockerConfigJson(host, username, password)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{v1.DockerConfigJsonKey: cfg}, nil
}

// deleteSecretIfExists deletes the secret at namespace/name, silently ignoring NotFound errors.
func (h *autoscalerHandler) deleteSecretIfExists(namespace, secretName string) error {
	err := h.secretClient.Delete(namespace, secretName, &metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// upsertSecret creates or updates a secret at namespace/secretName. If the secret already exists
// and its data is identical to the provided data, no write is performed.
func (h *autoscalerHandler) upsertSecret(namespace, secretName string, secretType v1.SecretType, data map[string][]byte, owner []metav1.OwnerReference) (*v1.Secret, error) {
	existing, err := h.secretCache.Get(namespace, secretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		if !reflect.DeepEqual(existing.Data, data) {
			updated := existing.DeepCopy()
			updated.Data = data
			return h.secretClient.Update(updated)
		}
		return existing, nil
	}
	return h.secretClient.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       namespace,
			OwnerReferences: owner,
		},
		Data: data,
		Type: secretType,
	})
}

// findGlobalClusterAutoScalerHostnameCreds iterates over globally configured pull secrets and
// returns the first username/password pair that covers the autoscaler chart repository host.
// When the global system default registry URL is not set, it falls back to searching the
// SystemDefaultRegistryPullSecrets directly, since those secrets may still contain credentials
// for the autoscaler chart repository even without a GSDR URL configured.
// Returns empty strings (no error) when no matching credentials are found.
func (h *autoscalerHandler) findGlobalClusterAutoScalerHostnameCreds() (string, string, error) {
	registry, _ := cluster.GetPrivateRegistry(nil)
	pullSecrets := cluster.GlobalPullSecretRefs()
	if registry != nil {
		pullSecrets = registry.PullSecrets
	}
	for _, ps := range pullSecrets {
		sec, err := h.secretCache.Get(ps.Namespace, ps.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return "", "", err
		}
		username, password, err := cluster.ExtractUsernamePasswordFromPullSecret(autoScalerChartRepositoryHost(), sec)
		if err != nil {
			if errors.Is(err, cluster.ErrRegistryHostnameNotFound) {
				continue
			}
			return "", "", err
		}
		return username, password, nil
	}
	return "", "", nil
}

// findClusterLevelAutoScalerHostnameCreds looks up the credentials for the autoscaler chart host
// from the provisioning cluster's registry configuration. The username and password is extracted from the first
// Auth secret found on the cluster which contains credentials for the autoscaler host.
func (h *autoscalerHandler) findClusterLevelAutoScalerHostnameCreds(provCluster *provv1.Cluster) (string, string, error) {
	chartHost := autoScalerChartRepositoryHost()
	ps := image.GetRegistryAuthSecretForHostname(provCluster, chartHost)
	if ps == "" {
		return "", "", nil
	}
	pullSecret, err := h.secretCache.Get(provCluster.Namespace, ps)
	if err != nil {
		return "", "", err
	}
	return cluster.ExtractUsernamePasswordFromPullSecret(chartHost, pullSecret)
}
