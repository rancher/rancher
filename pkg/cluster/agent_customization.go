package cluster

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

// GetClusterAgentTolerations returns additional tolerations for the cluster agent if they have been user defined. If
// not, nil is returned.
func GetClusterAgentTolerations(cluster *v3.Cluster) []corev1.Toleration {
	if cluster.Spec.ClusterAgentDeploymentCustomization != nil &&
		cluster.Spec.ClusterAgentDeploymentCustomization.AppendTolerations != nil {
		return cluster.Spec.ClusterAgentDeploymentCustomization.AppendTolerations
	}

	return nil
}

// GetClusterAgentAffinity returns node affinity for the cluster agent if it has been user defined. If not, then the
// default affinity is returned.
func GetClusterAgentAffinity(cluster *v3.Cluster) *corev1.Affinity {
	if cluster.Spec.ClusterAgentDeploymentCustomization != nil &&
		cluster.Spec.ClusterAgentDeploymentCustomization.OverrideAffinity != nil {
		return cluster.Spec.ClusterAgentDeploymentCustomization.OverrideAffinity
	}

	return settings.GetClusterAgentDefaultAffinity()
}

// GetClusterAgentResourceRequirements returns resource requirements (cpu, memory) for the cluster agent if it has been
// user defined. If not, nil is returned.
func GetClusterAgentResourceRequirements(cluster *v3.Cluster) *corev1.ResourceRequirements {
	if cluster.Spec.ClusterAgentDeploymentCustomization != nil &&
		cluster.Spec.ClusterAgentDeploymentCustomization.OverrideResourceRequirements != nil {
		return cluster.Spec.ClusterAgentDeploymentCustomization.OverrideResourceRequirements
	}

	return nil
}

// GetFleetAgentTolerations returns additional tolerations for the fleet agent if it has been user defined. If not,
// then nil is returned.
func GetFleetAgentTolerations(cluster *v3.Cluster) []corev1.Toleration {
	if cluster.Spec.FleetAgentDeploymentCustomization != nil &&
		cluster.Spec.FleetAgentDeploymentCustomization.AppendTolerations != nil {
		return cluster.Spec.FleetAgentDeploymentCustomization.AppendTolerations
	}

	return nil
}

// GetFleetAgentAffinity returns node affinity for the fleet agent if it has been user defined. If not, then the
// default affinity is returned.
func GetFleetAgentAffinity(cluster *v3.Cluster) *corev1.Affinity {
	if cluster.Spec.FleetAgentDeploymentCustomization != nil &&
		cluster.Spec.FleetAgentDeploymentCustomization.OverrideAffinity != nil {
		return cluster.Spec.FleetAgentDeploymentCustomization.OverrideAffinity
	}

	return settings.GetFleetAgentDefaultAffinity()
}

// GetFleetAgentResourceRequirements returns resource requirements (cpu, memory) for the fleet agent if it has been
// user defined. If not, nil is returned.
func GetFleetAgentResourceRequirements(cluster *v3.Cluster) *corev1.ResourceRequirements {
	if cluster.Spec.FleetAgentDeploymentCustomization != nil &&
		cluster.Spec.FleetAgentDeploymentCustomization.OverrideResourceRequirements != nil {
		return cluster.Spec.FleetAgentDeploymentCustomization.OverrideResourceRequirements
	}

	return nil
}
