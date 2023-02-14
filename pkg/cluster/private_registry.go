package cluster

import (
	"encoding/base64"
	"encoding/json"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

// GetPrivateRegistryURL returns the URL of the private registry specified. It will return the cluster level registry if
// one is found, or the system default registry if no cluster level registry is found. If either is not found, it will
// return an empty string.
func GetPrivateRegistryURL(cluster *v3.Cluster) string {
	registry := GetPrivateRegistry(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

// GetPrivateRegistry returns a PrivateRegistry entry (or nil if one is not found) for the given
// clusters.management.cattle.io/v3 object. If a cluster-level registry is not defined, it will return the system
// default registry if one exists.
func GetPrivateRegistry(cluster *v3.Cluster) *rketypes.PrivateRegistry {
	privateClusterLevelRegistry := GetPrivateClusterLevelRegistry(cluster)
	if privateClusterLevelRegistry != nil {
		return privateClusterLevelRegistry
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		return &rketypes.PrivateRegistry{
			URL: settings.SystemDefaultRegistry.Get(),
		}
	}
	return nil
}

// GetPrivateClusterLevelRegistry returns the cluster-level registry for the given clusters.management.cattle.io/v3
// object (or nil if one is not found).
func GetPrivateClusterLevelRegistry(cluster *v3.Cluster) *rketypes.PrivateRegistry {
	if cluster != nil && cluster.Spec.RancherKubernetesEngineConfig != nil && len(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries) > 0 {
		config := cluster.Spec.RancherKubernetesEngineConfig
		return &config.PrivateRegistries[0]
	}
	return nil
}

// GeneratePrivateRegistryEncodedDockerConfig generates a base64 encoded docker config JSON blob for the provided
// registry, and returns the registry url, the json credentials, and an error if one was encountered. If the cluster is
// nil or no registry is configured for an RKE1 or v2prov cluster, no registry url or json blob are returned, but there
// is no error returned, since not having a registry is not an error. If a registry is configured for the cluster such
// that we know what the URL is, but we do not have enough information to generate the auth config, we return the url,
// an empty string for the auth config, and no error, as we have determined where the private registry is, but the lack
// of secrets indicate to us that the registry does not need authentication to communicate. For RKE1, we attempt to
// utilize the ECR credential plugin if the corresponding secret exists, otherwise the RKE1 private registry secret is
// stored in the docker config JSON format, so no transformation is required. Otherwise, for v2prov clusters, we extract
// the username and password from the secret, and transform it into the expected docker config JSON format. This
// function should not be called with unmigrated clusters, although it is benign to call this function with assembler
// clusters, as the function will reassemble them anyway.
func GeneratePrivateRegistryEncodedDockerConfig(cluster *v3.Cluster, secretLister v1.SecretLister) (string, string, error) {
	var err error
	if cluster == nil {
		return "", "", nil
	}

	cluster = cluster.DeepCopy()
	// Only assemble ECR credential, for RKE1 private registries, the credential is stored in the correct format, and
	// v2prov secrets won't be assembled because they are in the fleet workspace namespace of the cluster (currently this
	// is always the "fleet-default" namespace)
	cluster.Spec, err = assemblers.AssemblePrivateRegistryECRCredential(cluster.Spec.ClusterSecrets.PrivateRegistryECRSecret, assemblers.ClusterType, cluster.Name, cluster.Spec, secretLister)
	if err != nil {
		return "", "", err
	}

	registrySecretName := cluster.GetSecret(v3.ClusterPrivateRegistrySecret)

	// Private registry will only be defined on the cluster if it is an RKE1 cluster, mgmt clusters generated from
	// provisioning clusters do not have a populated `RancherKubernetesEngineConfig`.
	if registry := GetPrivateRegistry(cluster); registry != nil {
		// check for RKE1 ECR credentials first
		if registry.ECRCredentialPlugin != nil {
			// generate ecr authConfig
			authConfig, err := util.ECRCredentialPlugin(registry.ECRCredentialPlugin, registry.URL)
			if err != nil {
				return registry.URL, "", err
			}
			encodedJSON, err := json.Marshal(authConfig)
			if err != nil {
				return registry.URL, "", err
			}
			return registry.URL, base64.URLEncoding.EncodeToString(encodedJSON), nil
		}

		// no private registry secret, generate authconfig based on existing fields
		if registrySecretName == "" {
			return registry.URL, "", nil
		}

		// check for the RKE1 registry secret next
		registrySecret, err := secretLister.Get(namespace.GlobalNamespace, registrySecretName)
		if err == nil {
			return registry.URL, base64.URLEncoding.EncodeToString(registrySecret.Data[corev1.DockerConfigJsonKey]), nil
		}
		if err != nil && !apierrors.IsNotFound(err) { // ignore secret not found errors as we need to check v2prov clusters
			return registry.URL, "", err
		}
	}

	// cluster.GetSecret("PrivateRegistryURL") will be empty if the cluster is
	// RKE1, imported, or RKE2 with no cluster level registry configured.
	// For RKE2 with a cluster level registry configured, this is the
	// only reference to the registry URL available on the v3.Cluster.
	// Without it, we cannot generate the registry credentials (.dockerconfigjson)
	registryURL := cluster.GetSecret(v3.ClusterPrivateRegistryURL)
	if registryURL == "" {
		return "", "", nil
	}

	if registrySecretName == "" { // no private registry configured
		return registryURL, "", nil
	}

	registrySecret, err := secretLister.Get(cluster.Spec.FleetWorkspaceName, registrySecretName)
	if err != nil {
		return registryURL, "", err
	}

	username := string(registrySecret.Data["username"])
	password := string(registrySecret.Data["password"])
	authConfig := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			registryURL: credentialprovider.DockerConfigEntry{
				Username: username,
				Password: password,
			},
		},
	}

	registryJSON, err := json.Marshal(authConfig)
	if err != nil {
		return registryURL, "", err
	}

	return registryURL, base64.URLEncoding.EncodeToString(registryJSON), nil
}
