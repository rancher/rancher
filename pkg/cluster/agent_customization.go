package cluster

import (
	"encoding/json"
	"fmt"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

const (
	PriorityClassName        = "cattle-cluster-agent-priority-class"
	PodDisruptionBudgetName  = "cattle-cluster-agent-pod-disruption-budget"
	PriorityClassDescription = "Rancher managed Priority Class for the cattle-cluster-agent"
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
func GetClusterAgentAffinity(cluster *v3.Cluster) (*corev1.Affinity, error) {
	if cluster.Spec.ClusterAgentDeploymentCustomization != nil &&
		cluster.Spec.ClusterAgentDeploymentCustomization.OverrideAffinity != nil {
		return cluster.Spec.ClusterAgentDeploymentCustomization.OverrideAffinity, nil
	}

	return unmarshalAffinity(settings.ClusterAgentDefaultAffinity.Get())
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
func GetFleetAgentAffinity(cluster *v3.Cluster) (*corev1.Affinity, error) {
	if cluster.Spec.FleetAgentDeploymentCustomization != nil &&
		cluster.Spec.FleetAgentDeploymentCustomization.OverrideAffinity != nil {
		return cluster.Spec.FleetAgentDeploymentCustomization.OverrideAffinity, nil
	}

	return unmarshalAffinity(settings.FleetAgentDefaultAffinity.Get())
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

// unmarshalAffinity returns an unmarshalled object of the v1 node affinity. If unable to be unmarshalled, it returns
// nil and an error.
func unmarshalAffinity(affinity string) (*corev1.Affinity, error) {
	var affinityObj corev1.Affinity
	err := json.Unmarshal([]byte(affinity), &affinityObj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal node affinity: %w", err)
	}

	return &affinityObj, nil
}

// GetAgentSchedulingCustomizationSpec returns the SchedulingCustomization field located in the cluster spec if it exists
func GetAgentSchedulingCustomizationSpec(cluster *v3.Cluster) *v3.AgentSchedulingCustomization {
	if cluster == nil {
		return nil
	}

	specCustomization := cluster.Spec.ClusterAgentDeploymentCustomization
	if specCustomization == nil {
		return nil
	}

	return specCustomization.SchedulingCustomization
}

// GetAgentSchedulingCustomizationStatus returns the SchedulingCustomization field located in the cluster status if it exists
func GetAgentSchedulingCustomizationStatus(cluster *v3.Cluster) *v3.AgentSchedulingCustomization {
	if cluster == nil {
		return nil
	}

	statusCustomization := cluster.Status.AppliedClusterAgentDeploymentCustomization
	if statusCustomization == nil {
		return nil
	}

	return statusCustomization.SchedulingCustomization
}

// AgentDeploymentCustomizationChanged determines if the ClusterAgentDeploymentCustomization spec field
// matches the values set in the status. The returned booleans indicates that either the desired Affinities,
// tolerations, or resource requirements have been changed (in that order).
// AgentDeploymentCustomizationChanged Does not indicate if any SchedulingCustomization options have been changed.
func AgentDeploymentCustomizationChanged(cluster *v3.Cluster) bool {
	if cluster == nil {
		return false
	}

	var specCustomization *v3.AgentDeploymentCustomization
	var statusCustomization *v3.AgentDeploymentCustomization

	if cluster.Spec.ClusterAgentDeploymentCustomization != nil {
		specCustomization = cluster.Spec.ClusterAgentDeploymentCustomization
	}

	if cluster.Status.AppliedClusterAgentDeploymentCustomization != nil {
		statusCustomization = cluster.Status.AppliedClusterAgentDeploymentCustomization
	}

	if specCustomization == nil && statusCustomization == nil {
		return false
	}

	if specCustomization == nil {
		return true
	}

	var affinitiesDiffer, tolerationsDiffer, resourcesDiffer bool

	if statusCustomization == nil {
		affinitiesDiffer = specCustomization.OverrideAffinity != nil
		tolerationsDiffer = specCustomization.AppendTolerations != nil
		resourcesDiffer = specCustomization.OverrideResourceRequirements != nil

		return affinitiesDiffer || tolerationsDiffer || resourcesDiffer
	}

	if specCustomization.AppendTolerations != nil && statusCustomization.AppendTolerations != nil {
		tolerationsDiffer = !reflect.DeepEqual(specCustomization.AppendTolerations, statusCustomization.AppendTolerations)
	}

	if specCustomization.OverrideAffinity != nil && statusCustomization.OverrideAffinity != nil {
		affinitiesDiffer = !reflect.DeepEqual(specCustomization.OverrideAffinity, statusCustomization.OverrideAffinity)
	}

	if specCustomization.OverrideResourceRequirements != nil && statusCustomization.OverrideResourceRequirements != nil {
		resourcesDiffer = !reflect.DeepEqual(specCustomization.OverrideResourceRequirements, statusCustomization.OverrideResourceRequirements)
	}

	return affinitiesDiffer || tolerationsDiffer || resourcesDiffer
}

// AgentSchedulingPodDisruptionBudgetChanged compares the cluster spec and status to determine if the
// cluster agent Pod Disruption Budget has been updated or deleted (in that order).
func AgentSchedulingPodDisruptionBudgetChanged(cluster *v3.Cluster) (bool, bool) {
	specCustomization := GetAgentSchedulingCustomizationSpec(cluster)
	statusCustomization := GetAgentSchedulingCustomizationStatus(cluster)

	if specCustomization == nil && statusCustomization == nil {
		return false, false
	}

	if specCustomization != nil && statusCustomization == nil {
		if specCustomization.PodDisruptionBudget != nil {
			return true, false
		}
		return false, false
	}

	if specCustomization == nil {
		if statusCustomization.PodDisruptionBudget != nil {
			return false, true
		}
		return false, false
	}

	return !reflect.DeepEqual(specCustomization.PodDisruptionBudget, statusCustomization.PodDisruptionBudget), specCustomization.PodDisruptionBudget == nil
}

// AgentSchedulingPriorityClassChanged compares the cluster spec and status to determine if the
// cluster agent Priority Class has been changed, created, or deleted (in that order).
func AgentSchedulingPriorityClassChanged(cluster *v3.Cluster) (bool, bool, bool) {
	specCustomization := GetAgentSchedulingCustomizationSpec(cluster)
	statusCustomization := GetAgentSchedulingCustomizationStatus(cluster)

	if specCustomization == nil && statusCustomization == nil {
		return false, false, false
	}

	if specCustomization != nil && statusCustomization == nil {
		if specCustomization.PriorityClass != nil {
			return false, true, false
		}
		return false, false, false
	}

	if specCustomization == nil {
		if statusCustomization.PriorityClass != nil {
			return false, false, true
		}
		return false, false, false
	}

	return !reflect.DeepEqual(specCustomization.PriorityClass, statusCustomization.PriorityClass), false, specCustomization.PriorityClass == nil
}

// AgentSchedulingCustomizationEnabled determines if scheduling customization has been defined for either the
// PriorityClass or PodDisruptionBudget. It returns two booleans, which indicate if the Priority Class has
// been defined, and if the Pod Disruption Budget has been defined.
func AgentSchedulingCustomizationEnabled(cluster *v3.Cluster) (bool, bool) {
	if cluster == nil {
		return false, false
	}

	specCustomization := GetAgentSchedulingCustomizationSpec(cluster)
	if specCustomization == nil {
		return false, false
	}

	pdbEnabled := specCustomization.PodDisruptionBudget != nil
	pcEnabled := specCustomization.PriorityClass != nil

	return pcEnabled, pdbEnabled
}

// GetDesiredPodDisruptionBudgetValues returns the minAvailable and maxUnavailable fields values in the agent SchedulingCustomization
// if they have been set. If both fields are set to zero, only a value for maxUnavailable will be returned, as Pod Disruption Budgets
// can only set one of the two fields at a time. The validating webhook ensures that both fields cannot be set on the cluster object prior
// to this function being invoked.
func GetDesiredPodDisruptionBudgetValues(cluster *v3.Cluster) (string, string) {
	if cluster == nil {
		return "", ""
	}

	agentCustomization := GetAgentSchedulingCustomizationSpec(cluster)
	if agentCustomization == nil {
		return "", ""
	}

	if agentCustomization.PodDisruptionBudget == nil {
		return "", ""
	}

	PDBMinAvailable := agentCustomization.PodDisruptionBudget.MinAvailable
	PDBMaxUnavailable := agentCustomization.PodDisruptionBudget.MaxUnavailable

	if (PDBMinAvailable == "" || PDBMinAvailable == "0") && PDBMaxUnavailable != "" {
		return "", PDBMaxUnavailable
	}

	if PDBMinAvailable != "" && (PDBMaxUnavailable == "" || PDBMaxUnavailable == "0") {
		return PDBMinAvailable, ""
	}

	return "", "0"
}

// GetDesiredPriorityClassValueAndPreemption returns the Priority Class Priority value and Preemption setting
// if configured in the agent SchedulingCustomization field.
func GetDesiredPriorityClassValueAndPreemption(cluster *v3.Cluster) (int, string) {
	if cluster == nil {
		return 0, ""
	}

	agentCustomization := GetAgentSchedulingCustomizationSpec(cluster)
	if agentCustomization == nil {
		return 0, ""
	}

	if agentCustomization.PriorityClass == nil {
		return 0, ""
	}

	var PCPreemption string
	if agentCustomization.PriorityClass.Preemption != nil {
		PCPreemption = string(*agentCustomization.PriorityClass.Preemption)
	}

	return agentCustomization.PriorityClass.Value, PCPreemption
}

// UpdateAppliedAgentDeploymentCustomization updates the cluster AppliedClusterAgentDeploymentCustomization Status
// field with the most recent configuration located in the Spec.
func UpdateAppliedAgentDeploymentCustomization(cluster *v3.Cluster) {
	if cluster == nil {
		return
	}

	agentCustomization := cluster.Spec.ClusterAgentDeploymentCustomization
	if agentCustomization == nil {
		cluster.Status.AppliedClusterAgentDeploymentCustomization = nil
		return
	}

	// If the deployment customization struct exists, but all of its
	// fields are empty, we should just clear it
	if agentCustomization.AppendTolerations == nil &&
		agentCustomization.OverrideAffinity == nil &&
		agentCustomization.OverrideResourceRequirements == nil &&
		agentCustomization.SchedulingCustomization == nil {

		cluster.Status.AppliedClusterAgentDeploymentCustomization = nil
		return
	}

	cluster.Status.AppliedClusterAgentDeploymentCustomization = &v3.AgentDeploymentCustomization{
		AppendTolerations:            agentCustomization.AppendTolerations,
		OverrideAffinity:             agentCustomization.OverrideAffinity,
		OverrideResourceRequirements: agentCustomization.OverrideResourceRequirements,
		SchedulingCustomization:      agentCustomization.SchedulingCustomization,
	}
}
