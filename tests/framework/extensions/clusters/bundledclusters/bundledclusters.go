package bundledclusters

import (
	"fmt"

	v3 "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
)

// BundledCluster is a struct that can contain different cluster version types and the meta of the cluster:
//   - Meta is a ClusterMeta value of cluster's meta.
//   - V1 is a v1 type cluster value of the cluster.
//   - V3 is a v3 type cluster value of the cluster.
type BundledCluster struct {
	Meta *clusters.ClusterMeta
	V1   *v1.SteveAPIObject
	V3   *v3.Cluster
}

// NewWithClusterMeta is a constructor to initialize a BundledCluster.
// It returns cluster meta struct and error if any.
// Cluster v1 and v3 versions can't store value at the same time.
func NewWithClusterMeta(cmeta *clusters.ClusterMeta) (cluster *BundledCluster, err error) {
	cluster = new(BundledCluster)

	clusterMeta := *cmeta

	isClusterImported := cmeta.IsImported
	isClusterRKE2 := cmeta.Provider == clusters.KubernetesProviderRKE2
	isClusterK3S := cmeta.Provider == clusters.KubernetesProviderK3S

	isClusterV1 := (isClusterK3S || isClusterRKE2) && isClusterImported

	if isClusterV1 {
		cluster.V1 = new(v1.SteveAPIObject)
		clusterMeta.ID = fmt.Sprintf("fleet-default/%v", clusterMeta.Name)
	} else if !isClusterV1 {
		cluster.V3 = new(v3.Cluster)
	}

	cluster.Meta = &clusterMeta

	return
}
