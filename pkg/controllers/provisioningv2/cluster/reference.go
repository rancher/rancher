package cluster

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	claimedLabelNamespace = "provisioning.cattle.io/claimed-by-namespace"
	claimedLabelName      = "provisioning.cattle.io/claimed-by-name"
)

func (h *handler) referenceCluster(cluster *v1.Cluster, status v1.ClusterStatus) ([]runtime.Object, v1.ClusterStatus, error) {
	rCluster, err := h.claimCluster(cluster, status)
	if err != nil {
		return nil, status, err
	}

	return h.updateStatus(nil, cluster, status, rCluster)
}

func (h *handler) claimCluster(cluster *v1.Cluster, status v1.ClusterStatus) (*v3.Cluster, error) {
	if status.ClusterName != "" {
		return h.mgmtClusterCache.Get(status.ClusterName)
	}

	if cluster.Spec.ReferencedConfig.ManagementClusterName == "" {
		return nil, fmt.Errorf("missing managementClusterName for referenced cluster %s/%s", cluster.Namespace, cluster.Name)
	}

	claimed, err := h.mgmtClusterCache.List(labels.SelectorFromSet(map[string]string{
		claimedLabelName:      cluster.Name,
		claimedLabelNamespace: cluster.Namespace,
	}))
	if err != nil {
		return nil, err
	}

	if len(claimed) > 1 {
		return nil, fmt.Errorf("more than one (%d) cluster is claimed by %s/%s remove %s and %s label on the undesired clusters",
			len(claimed), cluster.Namespace, cluster.Name, claimedLabelNamespace, claimedLabelName)
	}

	if len(claimed) == 1 {
		return claimed[0], nil
	}

	available, err := h.mgmtClusterCache.Get(cluster.Spec.ReferencedConfig.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	if available.Labels[claimedLabelName] != "" || available.Labels[claimedLabelNamespace] != "" {
		return nil, fmt.Errorf("cluster %s/%s is already claimed by %s/%s, can not claim for %s/%s",
			available.Namespace, available.Name,
			available.Labels[claimedLabelNamespace], available.Labels[claimedLabelName],
			cluster.Namespace, cluster.Name)
	}

	updated := available.DeepCopy()
	if updated.Labels == nil {
		updated.Labels = map[string]string{}
	}
	updated.Labels[claimedLabelName] = cluster.Name
	updated.Labels[claimedLabelNamespace] = cluster.Namespace
	return h.mgmtClusters.Update(updated)
}
