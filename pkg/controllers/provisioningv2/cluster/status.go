package cluster

import (
	"reflect"
	"strings"
	"sync"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/gvk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	clusterInfoQuietPeriod = 20 * time.Second
)

var lastClusterInfoReconcile sync.Map // map[clusterKey]time.Time for debouncing ClusterInfo field computations when needed

func (h *handler) updateClusterStatus(key string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name == "" {
		lastClusterInfoReconcile.Delete(key)
		return cluster, nil
	}

	var provCluster *v1.Cluster
	provClusters, err := h.clusterCache.GetByIndex(ByCluster, cluster.Name)
	if err != nil {
		return nil, err
	}
	if len(provClusters) > 0 {
		provCluster = provClusters[0]
	}

	provider := h.getMachineProvider(cluster, provCluster)
	if provider == "" && cluster.Status.Info != nil {
		provider = cluster.Status.Info.MachineProvider
	}

	var provisioningClusterRef *corev1.ObjectReference
	if cluster.Status.Info != nil {
		provisioningClusterRef = cluster.Status.Info.ProvisioningClusterRef
	}
	if provisioningClusterRef == nil && provCluster != nil && cluster.Annotations["provisioning.cattle.io/administrated"] == "true" {
		gvk, _ := gvk.Get(provCluster)
		provisioningClusterRef = &corev1.ObjectReference{
			Namespace: provCluster.Namespace,
			Name:      provCluster.Name,
		}
		provisioningClusterRef.APIVersion, provisioningClusterRef.Kind = gvk.ToAPIVersionAndKind()
	}

	kubernetesVersion := h.getKubernetesVersion(cluster, provCluster)
	k8sProvider := h.getKubernetesProvider(cluster, provisioningClusterRef, kubernetesVersion)

	nodeCount, err := h.getNodeCount(cluster, provCluster)
	if err != nil {
		return nil, err
	}

	// Debounce expensive fields, currently only arch
	arch := ""
	if cluster.Status.Info != nil {
		arch = cluster.Status.Info.Arch
	}
	if h.getClusterInfoWaitTime(key) == 0 {
		arch, err = h.getArch(cluster)
		if err != nil {
			return nil, err
		}
		lastClusterInfoReconcile.Store(key, time.Now())
	}

	desiredInfo := &v3.ClusterInfo{
		MachineProvider:        provider,
		KubernetesVersion:      kubernetesVersion,
		NodeCount:              nodeCount,
		Arch:                   arch,
		ProvisioningClusterRef: provisioningClusterRef,
	}

	if reflect.DeepEqual(cluster.Status.Info, desiredInfo) && cluster.Status.Provider == k8sProvider {
		return cluster, nil
	}

	clusterCopy := cluster.DeepCopy()
	clusterCopy.Status.Info = desiredInfo
	clusterCopy.Status.Provider = k8sProvider
	return h.mgmtClusters.UpdateStatus(clusterCopy)
}

// getKubernetesVersion returns the Kubernetes version of the cluster. It first checks status.kubeVersion for the actual running version. If empty (e.g. during
// initial provisioning), it falls back to the version defined in the spec for UI/UX display purposes to show the intended version.
func (h *handler) getKubernetesVersion(cluster *v3.Cluster, provCluster *v1.Cluster) string {
	if cluster.Status.Version != nil && cluster.Status.Version.GitVersion != "" {
		return cluster.Status.Version.GitVersion
	}
	if provCluster != nil && provCluster.Spec.RKEConfig != nil && provCluster.Spec.KubernetesVersion != "" {
		return provCluster.Spec.KubernetesVersion
	}
	if cluster.Spec.AKSConfig != nil && cluster.Spec.AKSConfig.KubernetesVersion != nil {
		return *cluster.Spec.AKSConfig.KubernetesVersion
	}
	if cluster.Spec.EKSConfig != nil && cluster.Spec.EKSConfig.KubernetesVersion != nil {
		return *cluster.Spec.EKSConfig.KubernetesVersion
	}
	if cluster.Spec.GKEConfig != nil && cluster.Spec.GKEConfig.KubernetesVersion != nil {
		return *cluster.Spec.GKEConfig.KubernetesVersion
	}
	if cluster.Spec.AliConfig != nil {
		return cluster.Spec.AliConfig.KubernetesVersion
	}
	if cluster.Spec.GenericEngineConfig != nil {
		if version, ok := (*cluster.Spec.GenericEngineConfig)["kubernetesVersion"].(string); ok {
			return version
		}
	}
	return ""
}

// getMachineProvider returns the infrastructure provider for the cluster.
// Note: This differs from other status fields:
// - status.Provider: The K8s distribution (e.g., RKE2, K3s) detected once the cluster is ready.
// - status.Driver: The engine/cluster driver (e.g., "rke", "aks/eks/gke", "imported").
// status.MachineProvider is intended to indicate the infrastructure provider when possible or cluster type otherwise - currently used for UI/UX purposes.
// 1. For v2prov clusters: Derived from the machine config (e.g., Amazon, DO) for machine pools or "custom" for non-machine pool clusters.
// 2. For local clusters: Explicitly set to "local" (vs "imported") to identify the management cluster.
// 3. For imported clusters: Defaults to "imported" (future: may be enhanced for CAPI-specific providers).
// 4. For hosted clusters: Could be derived from the cluster spec (aks/gke/eks/ali) or "imported" if spec.*Config.Imported is true.
func (h *handler) getMachineProvider(cluster *v3.Cluster, provCluster *v1.Cluster) string {
	if provCluster != nil {
		if val, ok := provCluster.Annotations["ui.rancher/provider"]; ok {
			return val
		}

		if provCluster.Spec.RKEConfig != nil {
			if len(provCluster.Spec.RKEConfig.MachinePools) == 0 {
				return "custom"
			}
			pool := provCluster.Spec.RKEConfig.MachinePools[0]
			if pool.NodeConfig == nil || pool.NodeConfig.Kind == "" {
				return ""
			}
			return strings.TrimSuffix(pool.NodeConfig.Kind, "Config")
		}
	}

	if cluster.Annotations["provisioning.cattle.io/administrated"] == "true" {
		// couldn't find provCluster for v2prov clusters, will set provider=imported incorrectly if we don't return here.
		return ""
	}

	if cluster.Spec.Internal {
		return "local"
	}

	if cluster.Labels["provider.cattle.io"] == "harvester" || cluster.Status.Provider == "harvester" {
		return "harvester"
	}

	if cluster.Spec.AKSConfig != nil {
		if cluster.Spec.AKSConfig.Imported {
			return "imported"
		}
		return "aks"
	}
	if cluster.Spec.EKSConfig != nil {
		if cluster.Spec.EKSConfig.Imported {
			return "imported"
		}
		return "eks"
	}
	if cluster.Spec.GKEConfig != nil {
		if cluster.Spec.GKEConfig.Imported {
			return "imported"
		}
		return "gke"
	}
	if cluster.Spec.AliConfig != nil {
		if cluster.Spec.AliConfig.Imported {
			return "imported"
		}
		return "ali"
	}
	if cluster.Spec.GenericEngineConfig != nil {
		driverName, ok := (*cluster.Spec.GenericEngineConfig)["driverName"].(string)
		if ok {
			return driverName
		}
	}

	if cluster.Status.Driver == "" || (cluster.Status.Provider == cluster.Status.Driver && (cluster.Status.Driver == "rke2" || cluster.Status.Driver == "k3s")) {
		return "imported"
	}
	return ""
}

func (h *handler) getKubernetesProvider(cluster *v3.Cluster, provisioningClusterRef *corev1.ObjectReference, kubernetesVersion string) string {
	if cluster.Status.Provider != "" {
		return cluster.Status.Provider
	}
	// provisioningClusterRef is set only for v2prov custom/provisioned clusters, so we can reasonably detect provider based on kubernetesVersion.
	// For other cluster types, provider will be set by kubernetesProvider controller after cluster is ready.
	if provisioningClusterRef != nil && kubernetesVersion != "" {
		if strings.Contains(kubernetesVersion, "+rke2") {
			return "rke2"
		}
		if strings.Contains(kubernetesVersion, "+k3s") {
			return "k3s"
		}
	}
	return ""
}

func (h *handler) getNodeCount(cluster *v3.Cluster, provCluster *v1.Cluster) (int, error) {
	// There's usually lag between status.NodeCount and actual number of nodes for v2prov clusters, so checking capi machines cache for actual count.
	if provCluster != nil && provCluster.Spec.RKEConfig != nil && h.capiMachinesCache != nil {
		machines, err := h.capiMachinesCache.List(provCluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: provCluster.Name}))
		if err != nil {
			return 0, err
		}
		return len(machines), nil
	}

	// management.cattle.io.nodepool is removed
	return cluster.Status.NodeCount, nil
}

func (h *handler) getArch(cluster *v3.Cluster) (string, error) {
	machines, err := h.mgmtNodesCache.List(cluster.Name, labels.Everything())
	if err != nil {
		return "", err
	}
	arch := ""
	for _, machine := range machines {
		if machine.Status.NodeLabels[corev1.LabelArchStable] != "" {
			if arch == "" {
				arch = machine.Status.NodeLabels[corev1.LabelArchStable]
			} else if arch != machine.Status.NodeLabels[corev1.LabelArchStable] {
				return "mixed", nil
			}
		}
	}
	return arch, nil
}

func (h *handler) getClusterInfoWaitTime(key string) time.Duration {
	if v, ok := lastClusterInfoReconcile.Load(key); ok {
		lastReconcile := v.(time.Time)
		if wait := clusterInfoQuietPeriod - time.Since(lastReconcile); wait > 0 {
			return wait
		}
	}
	return 0
}
