package clusters

import (
	"github.com/pkg/errors"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
)

// KubernetesProvider is a string type to determine cluster's provider.
type KubernetesProvider string

const (
	rke  = "rke"
	k3s  = "k3s"
	rke2 = "rke2"
	aks  = "aks"
	eks  = "eks"
	gke  = "gke"

	// KubernetesProvider string enums are to determine cluster's provider.
	KubernetesProviderRKE  KubernetesProvider = rke
	KubernetesProviderRKE2 KubernetesProvider = rke2
	KubernetesProviderK3S  KubernetesProvider = k3s
	KubernetesProviderAKS  KubernetesProvider = aks
	KubernetesProviderEKS  KubernetesProvider = eks
	KubernetesProviderGKE  KubernetesProvider = gke
)

// ClusterMeta is a struct that contains a cluster's meta
type ClusterMeta struct {
	// ID used for value of cluster's ID
	ID string

	// Name used for cluster's name.
	Name string

	// Provider is used for cluster's provider.
	Provider KubernetesProvider

	// IsHosted is used for cluster's hosted information.
	IsHosted bool

	// IsImported is used for cluster's imported information.
	IsImported bool
}

// NewClusterMeta is a function to initialize new ClusterMeta for a specific cluster.
func NewClusterMeta(client *rancher.Client, clusterName string) (clusterMeta *ClusterMeta, err error) {
	clusterMeta = new(ClusterMeta)
	clusterMeta.Name = clusterName

	clusterMeta.ID, err = GetClusterIDByName(client, clusterName)
	if err != nil {
		return
	}

	clusterMeta.Provider, err = GetClusterProvider(client, clusterMeta.ID)
	if err != nil {
		return
	}

	clusterMeta.IsImported, err = IsClusterImported(client, clusterMeta.ID)
	if err != nil {
		return
	}

	clusterMeta.IsHosted = IsHostedProvider(clusterMeta.Provider)

	return
}

// GetClusterProvider is a function to get cluster's KubernetesProvider.
func GetClusterProvider(client *rancher.Client, clusterID string) (provider KubernetesProvider, err error) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return
	}

	switch cluster.Provider {
	case rke:
		provider = KubernetesProviderRKE
	case k3s:
		provider = KubernetesProviderK3S
	case rke2:
		provider = KubernetesProviderRKE2
	case aks:
		provider = KubernetesProviderAKS
	case eks:
		provider = KubernetesProviderEKS
	case gke:
		provider = KubernetesProviderGKE
	default:
		return "", errors.Wrap(err, "invalid cluster provider")
	}

	return
}

// IsHostedProvider is a function to get a boolean value about if the cluster is hosted or not.
func IsHostedProvider(provider KubernetesProvider) (isHosted bool) {
	if provider == KubernetesProviderAKS || provider == KubernetesProviderGKE || provider == KubernetesProviderEKS {
		return true
	}

	return
}
