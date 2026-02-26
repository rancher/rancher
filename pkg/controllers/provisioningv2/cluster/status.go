package cluster

import (
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
)

func (h *handler) updateClusterStatus(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name == "" {
		return cluster, nil
	}

	var provCluster *v1.Cluster
	provClusters, err := h.clusterCache.GetByIndex(ByCluster, cluster.Name)
	if err == nil && len(provClusters) > 0 {
		provCluster = provClusters[0]
	}

	provider := h.getMachineProvider(cluster, provCluster)
	kubernetesVersion := h.getKubernetesVersion(cluster, provCluster)

	if cluster.Status.MachineProvider == provider && cluster.Status.KubernetesVersion == kubernetesVersion {
		return cluster, nil
	}

	clusterCopy := cluster.DeepCopy()
	clusterCopy.Status.MachineProvider = provider
	clusterCopy.Status.KubernetesVersion = kubernetesVersion

	return h.mgmtClusters.Update(clusterCopy)

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
			if len(provCluster.Spec.RKEConfig.MachinePools) > 0 {
				pool := provCluster.Spec.RKEConfig.MachinePools[0]
				if pool.NodeConfig == nil || pool.NodeConfig.Kind == "" {
					return ""
				}
				return strings.TrimSuffix(pool.NodeConfig.Kind, "Config")
			}
		}
	}

	if cluster.Labels["provider.cattle.io"] == "harvester" || cluster.Status.Provider == "harvester" {
		return "harvester"
	}

	if cluster.Spec.Internal {
		return "local"
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
