package cluster

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/fleet"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	kcorev1 "k8s.io/api/core/v1"
)

const (
	// SourcePullSecretLabel is used to track secrets in the cattle-system namespace which are used
	// as the global system default registry pull secrets (via the global setting).
	SourcePullSecretLabel = "management.cattle.io/rancher-default-registry-pull-secret"
	// CopiedPullSecretLabel is used to track copies of the source pull secrets,
	// so that their life cycle can be managed by other controllers (e.g. imported cluster cleanup).
	CopiedPullSecretLabel = "management.cattle.io/rancher-managed-pull-secret"
)

var MgmtNameRegexp = regexp.MustCompile("^(c-[a-z0-9]{5}|local)$")

// AgentPullSecret represents a single pull secret used for agent image pulls.
type AgentPullSecret struct {
	// Name is the name of the Kubernetes Secret containing the registry credentials for this pull secret.
	Name string
	// DockerConfigJSON is the base64 encoded .dockerconfigjson content that should be used when pulling from the private registry.
	DockerConfigJSON string
}

type PrivateRegistry struct {
	// URL is the hostname of the private registry. This value should not include any protocols (e.g. https://) or ports.
	URL string
	// PullSecrets are a slice of object references to secrets in the relevant cluster.
	PullSecrets []kcorev1.SecretReference
}

func (p *PrivateRegistry) PullSecretNamesAsSlice() []string {
	var out []string
	for _, secret := range p.PullSecrets {
		out = append(out, secret.Name)
	}
	return out
}

func (p *PrivateRegistry) PullSecretsAsObjectReferences() []kcorev1.LocalObjectReference {
	var out []kcorev1.LocalObjectReference
	for _, secret := range p.PullSecrets {
		out = append(out, kcorev1.LocalObjectReference{
			Name: secret.Name,
		})
	}
	return out
}

// GetPrivateRegistryURL returns the URL of the private registry specified. It will return the cluster level registry if
// one is found, or the global system default registry if no cluster level registry is found. If neither is found, it will
// return an empty string.
func GetPrivateRegistryURL(cluster *v3.Cluster) string {
	registry, _ := GetPrivateRegistry(cluster)
	if registry == nil {
		return ""
	}
	return registry.URL
}

// GetPrivateRegistry returns a PrivateRegistry entry (or nil if one is not found) for the given
// clusters.management.cattle.io/v3 object. If a cluster-level registry is not defined, or
// the provided cluster is nil, it will return the global system default registry configuration if it exists.
func GetPrivateRegistry(importedOrHostedCluster *v3.Cluster) (registry *PrivateRegistry, isGlobalDefault bool) {
	if clr := GetImportedPrivateClusterLevelRegistry(importedOrHostedCluster); clr != nil {
		return clr, false
	}
	return getDefaultRegistryConfiguration("", ""), true
}

func getDefaultRegistryConfiguration(registryURL, pullSecretNames string) *PrivateRegistry {
	if registryURL == "" {
		registryURL = settings.SystemDefaultRegistry.Get()
	}
	if registryURL == "" {
		return nil
	}

	if pullSecretNames == "" {
		pullSecretNames = settings.SystemDefaultRegistryPullSecrets.Get()
	}

	var globalSecrets []kcorev1.SecretReference
	for _, pullSecret := range strings.Split(pullSecretNames, ",") {
		pullSecret = strings.TrimSpace(pullSecret)
		if pullSecret == "" {
			continue
		}
		globalSecrets = append(globalSecrets, kcorev1.SecretReference{
			Namespace: namespaces.System,
			Name:      pullSecret,
		})
	}

	return &PrivateRegistry{
		URL:         registryURL,
		PullSecrets: globalSecrets,
	}
}

// GetImportedPrivateClusterLevelRegistry returns the cluster-level registry for the given clusters.management.cattle.io/v3
// object (or nil if one is not found).
func GetImportedPrivateClusterLevelRegistry(cluster *v3.Cluster) *PrivateRegistry {
	if cluster == nil {
		return nil
	}

	importedCfg := cluster.Spec.ImportedConfig
	if importedCfg == nil {
		// falls back to global configuration
		return nil
	}

	url := importedCfg.PrivateRegistryURL
	secrets := importedCfg.PrivateRegistryPullSecrets
	if url == "" {
		return nil
	}

	ns := cluster.Spec.FleetWorkspaceName
	if ns == "" {
		ns = fleet.ClustersDefaultNamespace
	}

	var pullSecrets []kcorev1.SecretReference
	if len(secrets) > 0 {
		for _, pullSecret := range secrets {
			pullSecrets = append(pullSecrets, kcorev1.SecretReference{
				Namespace: ns,
				Name:      pullSecret,
			})
		}
	}

	return &PrivateRegistry{
		URL:         url,
		PullSecrets: pullSecrets,
	}
}

// GeneratePrivateRegistryEncodedDockerConfig generates one or more AgentPullSecret
// for the provided cluster.
//
// The function returns ("", nil, nil) when:
//   - the cluster is nil
//   - no registry is configured at either the global or cluster level
//
// Provisioned v2 clusters (rke2/k3s):
//   - Uses .spec.ClusterSecrets to determine the cluster level registry URL and auth secret. Returns the registry URL and a single AgentPullSecret
//
// Imported/hosted clusters:
//   - Uses .spec.ImportedConfig to determine the cluster level registry URL and all auth secrets configured. Returns the registry URL and one or more AgentPullSecret
func GeneratePrivateRegistryEncodedDockerConfig(cluster *v3.Cluster, secretLister v1.SecretLister) (string, []AgentPullSecret, error) {
	if cluster == nil {
		return "", nil, nil
	}

	// cluster.GetSecret("PrivateRegistryURL") will only be populated for provisioned
	// rke2/k3s clusters which have defined a system default registry, either at the cluster level
	// or by inheriting the global system default registry configuration setup in Rancher.
	// Imported and hosted clusters will not have these fields set, as they are only populated by the provv2 generating handlers.
	// This field is the only reference to the cluster level registry URL for v2prov clusters.
	if cluster.GetSecret(v3.ClusterPrivateRegistryURL) != "" {
		return generateProvisionedClusterDockerConfig(cluster, secretLister)
	}

	// Otherwise, look elsewhere on the v3 cluster for registry info.
	// This will also return the global system default registry configuration if the cluster
	// doesn't provide any overrides.
	if systemDefaultRegistry, _ := GetPrivateRegistry(cluster); systemDefaultRegistry != nil {
		return generateImportedClusterDockerConfig(cluster, secretLister, systemDefaultRegistry)
	}

	// no registry configured
	return "", nil, nil
}

func generateProvisionedClusterDockerConfig(cluster *v3.Cluster, secretLister v1.SecretLister) (string, []AgentPullSecret, error) {
	v2ProvRegistryURL := cluster.GetSecret(v3.ClusterPrivateRegistryURL)

	// The PrivateRegistrySecret has the same name both for v1 or v2 provisioning clusters despite being in different areas of the spec
	registrySecretName := cluster.GetSecret(v3.ClusterPrivateRegistrySecret)
	if registrySecretName == "" {
		return v2ProvRegistryURL, nil, nil
	}

	registrySecret, err := secretLister.Get(cluster.Spec.FleetWorkspaceName, registrySecretName)
	if err != nil {
		return v2ProvRegistryURL, nil, err
	}

	configJson, err := ConvertToDockerConfigJson(v2ProvRegistryURL, registrySecret)
	if err != nil {
		return "", nil, fmt.Errorf("clusterDeploy: failed to convert pull secret to json for cluster %s: %w", cluster.Name, err)
	}

	// note:
	//       Provisioned rke2/k3s clusters only support a single image pull secret,
	//       additional registry credentials are passed using the containerd configuration
	//       delivered by the planner.
	return v2ProvRegistryURL, []AgentPullSecret{{
		Name:             "cattle-private-registry",
		DockerConfigJSON: base64.StdEncoding.EncodeToString(configJson),
	}}, nil
}

func generateImportedClusterDockerConfig(cluster *v3.Cluster, secretLister v1.SecretLister, registry *PrivateRegistry) (string, []AgentPullSecret, error) {
	clusterSystemDefaultURL := registry.URL
	// Only generate credentials for imported or hosted clusters.
	if !MgmtNameRegexp.MatchString(cluster.Name) {
		return clusterSystemDefaultURL, nil, nil
	}

	if len(registry.PullSecrets) == 0 {
		return clusterSystemDefaultURL, nil, nil
	}
	var pullSecrets []AgentPullSecret

	// build out all the cluster level pull secrets
	for _, pullSecret := range registry.PullSecrets {
		sec, err := secretLister.Get(pullSecret.Namespace, pullSecret.Name)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get pull secret %s in namespace %s for cluster %s: %w", pullSecret.Name, pullSecret.Namespace, cluster.Name, err)
		}

		configJson, err := ConvertToDockerConfigJson(clusterSystemDefaultURL, sec)
		if err != nil {
			return "", nil, fmt.Errorf("clusterDeploy: failed to convert pull secret to json: %w", err)
		}

		pullSecrets = append(pullSecrets, AgentPullSecret{
			Name:             sec.Name,
			DockerConfigJSON: base64.StdEncoding.EncodeToString(configJson),
		})
	}

	return clusterSystemDefaultURL, pullSecrets, nil
}
