package cluster

import (
	"encoding/json"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
)

func (h *handler) updateV1SchedulingCustomization(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}

	if !features.ClusterAgentSchedulingCustomization.Enabled() {
		return cluster, nil
	}

	cluster, err := h.updateV1AgentSchedulingCustomization(cluster)
	if err != nil {
		return nil, err
	}

	cluster, err = h.updateV1FleetAgentSchedulingCustomization(cluster)
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func (h *handler) updateV3SchedulingCustomization(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}

	if !features.ClusterAgentSchedulingCustomization.Enabled() {
		return cluster, nil
	}

	cluster, err := h.updateV3AgentSchedulingCustomization(cluster)
	if err != nil {
		return nil, err
	}

	cluster, err = h.updateV3FleetAgentSchedulingCustomization(cluster)
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// updateV1AgentSchedulingCustomization looks for the
// provisioning.cattle.io/enable-scheduling-customization annotation on the v1.Cluster and if found
// populates the spec.ClusterAgentDeploymentCustomization.SchedulingCustomization fields with the
// default values set in the global settings. If the cluster-agent-scheduling-customization feature
// is disabled, the cluster will be returned unchanged. The
// provisioning.cattle.io/enable-scheduling-customization annotation can be set to 'true' or
// 'false', which will add or remove the scheduling customization field as needed.
func (h *handler) updateV1AgentSchedulingCustomization(cluster *v1.Cluster) (*v1.Cluster, error) {
	value, ok := cluster.Annotations[manageSchedulingDefaultsAnn]
	if !ok {
		return cluster, nil
	}

	lowerVal := strings.ToLower(value)
	if lowerVal != "true" && lowerVal != "false" {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	if lowerVal == "false" {
		delete(cluster.Annotations, manageSchedulingDefaultsAnn)

		if cluster.Spec.ClusterAgentDeploymentCustomization != nil {
			cluster.Spec.ClusterAgentDeploymentCustomization.SchedulingCustomization = nil
		}

		return h.clusters.Update(cluster)
	}

	// annotation was added to a cluster that already has the fields set, we should not override the existing values.
	if cluster.Spec.ClusterAgentDeploymentCustomization != nil && cluster.Spec.ClusterAgentDeploymentCustomization.SchedulingCustomization != nil {
		delete(cluster.Annotations, manageSchedulingDefaultsAnn)
		return h.clusters.Update(cluster)
	}

	defaultPC, defaultPDB, err := getDefaultSchedulingCustomization[v1.PriorityClassSpec, v1.PodDisruptionBudgetSpec]()
	if err != nil {
		return cluster, fmt.Errorf("failed to get default scheduling customization: %w", err)
	}
	if defaultPDB != nil || defaultPC != nil {
		if cluster.Spec.ClusterAgentDeploymentCustomization == nil {
			cluster.Spec.ClusterAgentDeploymentCustomization = &v1.AgentDeploymentCustomization{}
		}
		cluster.Spec.ClusterAgentDeploymentCustomization.SchedulingCustomization = &v1.AgentSchedulingCustomization{
			PodDisruptionBudget: defaultPDB,
			PriorityClass:       defaultPC,
		}
	}

	delete(cluster.Annotations, manageSchedulingDefaultsAnn)
	return h.clusters.Update(cluster)
}

// updateV1FleetAgentSchedulingCustomization looks for the
// provisioning.cattle.io/enable-scheduling-customization annotation on the v1.Cluster and if found
// populates the spec.FleetAgentDeploymentCustomization.SchedulingCustomization fields with the
// default values set in the global settings. If the cluster-agent-scheduling-customization feature
// is disabled, the cluster will be returned unchanged. The
// provisioning.cattle.io/enable-fleet-scheduling-customization annotation can be set to 'true' or
// 'false', which will add or remove the scheduling customization field as needed.
func (h *handler) updateV1FleetAgentSchedulingCustomization(cluster *v1.Cluster) (*v1.Cluster, error) {
	value, ok := cluster.Annotations[manageFleetSchedulingDefaultsAnn]
	if !ok {
		return cluster, nil
	}

	lowerVal := strings.ToLower(value)
	if lowerVal != "true" && lowerVal != "false" {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	if lowerVal == "false" {
		delete(cluster.Annotations, manageFleetSchedulingDefaultsAnn)

		if cluster.Spec.FleetAgentDeploymentCustomization != nil {
			cluster.Spec.FleetAgentDeploymentCustomization.SchedulingCustomization = nil
		}

		return h.clusters.Update(cluster)
	}

	// annotation was added to a cluster that already has the fields set, we should not override the existing values.
	if cluster.Spec.FleetAgentDeploymentCustomization != nil && cluster.Spec.FleetAgentDeploymentCustomization.SchedulingCustomization != nil {
		delete(cluster.Annotations, manageFleetSchedulingDefaultsAnn)
		return h.clusters.Update(cluster)
	}

	defaultPC, defaultPDB, err := getDefaultFleetSchedulingCustomization[v1.PriorityClassSpec, v1.PodDisruptionBudgetSpec]()
	if err != nil {
		return cluster, fmt.Errorf("failed to get default scheduling customization: %w", err)
	}
	if defaultPDB != nil || defaultPC != nil {
		if cluster.Spec.FleetAgentDeploymentCustomization == nil {
			cluster.Spec.FleetAgentDeploymentCustomization = &v1.AgentDeploymentCustomization{}
		}
		cluster.Spec.FleetAgentDeploymentCustomization.SchedulingCustomization = &v1.AgentSchedulingCustomization{
			PodDisruptionBudget: defaultPDB,
			PriorityClass:       defaultPC,
		}
	}

	delete(cluster.Annotations, manageFleetSchedulingDefaultsAnn)
	return h.clusters.Update(cluster)
}

// updateV3AgentSchedulingCustomization looks for the provisioning.cattle.io/enable-scheduling-customization annotation on
// the v3.Cluster and if found populates the spec.ClusterAgentDeploymentCustomization.SchedulingCustomization and
// spec.FleetAgentDeploymentCustomization.SchedulingCustomization fields with the default values set in the global
// settings. If the cluster-agent-scheduling-customization feature is disabled, the cluster will be returned unchanged.
// The provisioning.cattle.io/enable-scheduling-customization annotation can be set to 'true' or 'false', which will add
// or remove the scheduling customization field as needed. updateV3AgentSchedulingCustomization is intended to handle KEv2
// and legacy clusters specifically.
func (h *handler) updateV3AgentSchedulingCustomization(cluster *v3.Cluster) (*v3.Cluster, error) {
	if !h.isLegacyCluster(cluster) {
		return nil, nil
	}

	value, ok := cluster.Annotations[manageSchedulingDefaultsAnn]
	if !ok {
		return cluster, nil
	}

	lowerVal := strings.ToLower(value)
	if lowerVal != "true" && lowerVal != "false" {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	if lowerVal == "false" {
		delete(cluster.Annotations, manageSchedulingDefaultsAnn)

		if cluster.Spec.ClusterAgentDeploymentCustomization != nil {
			cluster.Spec.ClusterAgentDeploymentCustomization.SchedulingCustomization = nil
		}

		return h.mgmtClusters.Update(cluster)
	}

	// annotation was added to a cluster that already has the fields set, we should not override the existing values.
	if cluster.Spec.ClusterAgentDeploymentCustomization != nil && cluster.Spec.ClusterAgentDeploymentCustomization.SchedulingCustomization != nil {
		delete(cluster.Annotations, manageSchedulingDefaultsAnn)
		return h.mgmtClusters.Update(cluster)
	}

	defaultPC, defaultPDB, err := getDefaultSchedulingCustomization[v3.PriorityClassSpec, v3.PodDisruptionBudgetSpec]()
	if err != nil {
		return cluster, fmt.Errorf("failed to get default scheduling customization: %w", err)
	}
	if defaultPDB != nil || defaultPC != nil {
		if cluster.Spec.ClusterAgentDeploymentCustomization == nil {
			cluster.Spec.ClusterAgentDeploymentCustomization = &v3.AgentDeploymentCustomization{}
		}
		cluster.Spec.ClusterAgentDeploymentCustomization.SchedulingCustomization = &v3.AgentSchedulingCustomization{
			PodDisruptionBudget: defaultPDB,
			PriorityClass:       defaultPC,
		}
	}

	delete(cluster.ObjectMeta.Annotations, manageSchedulingDefaultsAnn)
	return h.mgmtClusters.Update(cluster)
}

// updateV3FleetAgentSchedulingCustomization looks for the
// provisioning.cattle.io/enable-scheduling-customization annotation on the v3.Cluster and if found
// populates the spec.FleetAgentDeploymentCustomization.SchedulingCustomization fields with the
// default values set in the global settings. If the cluster-agent-scheduling-customization feature
// is disabled, the cluster will be returned unchanged. The
// provisioning.cattle.io/enable-scheduling-customization annotation can be set to 'true' or
// 'false', which will add or remove the scheduling customization field as needed.
// updateV3FleetAgentSchedulingCustomization is intended to handle KEv2 and legacy clusters specifically.
func (h *handler) updateV3FleetAgentSchedulingCustomization(cluster *v3.Cluster) (*v3.Cluster, error) {
	if !h.isLegacyCluster(cluster) {
		return nil, nil
	}

	value, ok := cluster.Annotations[manageFleetSchedulingDefaultsAnn]
	if !ok {
		return cluster, nil
	}

	lowerVal := strings.ToLower(value)
	if lowerVal != "true" && lowerVal != "false" {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	if lowerVal == "false" {
		delete(cluster.Annotations, manageFleetSchedulingDefaultsAnn)

		if cluster.Spec.FleetAgentDeploymentCustomization != nil {
			cluster.Spec.FleetAgentDeploymentCustomization.SchedulingCustomization = nil
		}

		return h.mgmtClusters.Update(cluster)
	}

	// annotation was added to a cluster that already has the fields set, we should not override the existing values.
	if cluster.Spec.FleetAgentDeploymentCustomization != nil && cluster.Spec.FleetAgentDeploymentCustomization.SchedulingCustomization != nil {
		delete(cluster.Annotations, manageFleetSchedulingDefaultsAnn)
		return h.mgmtClusters.Update(cluster)
	}

	defaultPC, defaultPDB, err := getDefaultFleetSchedulingCustomization[v3.PriorityClassSpec, v3.PodDisruptionBudgetSpec]()
	if err != nil {
		return cluster, fmt.Errorf("failed to get default scheduling customization: %w", err)
	}
	if defaultPDB != nil || defaultPC != nil {
		if cluster.Spec.FleetAgentDeploymentCustomization == nil {
			cluster.Spec.FleetAgentDeploymentCustomization = &v3.AgentDeploymentCustomization{}
		}
		cluster.Spec.FleetAgentDeploymentCustomization.SchedulingCustomization = &v3.AgentSchedulingCustomization{
			PodDisruptionBudget: defaultPDB,
			PriorityClass:       defaultPC,
		}
	}

	delete(cluster.ObjectMeta.Annotations, manageFleetSchedulingDefaultsAnn)
	return h.mgmtClusters.Update(cluster)
}

func getDefaultSchedulingCustomization[T v1.PriorityClassSpec | v3.PriorityClassSpec, TT v1.PodDisruptionBudgetSpec | v3.PodDisruptionBudgetSpec]() (*T, *TT, error) {
	defaultPC := settings.ClusterAgentDefaultPriorityClass.Get()
	pc := new(T)
	if defaultPC != "" {
		err := json.Unmarshal([]byte(defaultPC), &pc)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal default cluster agent priority class: %w", err)
		}
	} else {
		pc = nil
	}

	defaultPdb := settings.ClusterAgentDefaultPodDisruptionBudget.Get()
	pdb := new(TT)
	if defaultPdb != "" {
		err := json.Unmarshal([]byte(defaultPdb), &pdb)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal default cluster agent priority class: %w", err)
		}
	} else {
		pdb = nil
	}

	return pc, pdb, nil
}

func getDefaultFleetSchedulingCustomization[T v1.PriorityClassSpec | v3.PriorityClassSpec, TT v1.PodDisruptionBudgetSpec | v3.PodDisruptionBudgetSpec]() (*T, *TT, error) {
	defaultPC := settings.FleetAgentDefaultPriorityClass.Get()
	pc := new(T)
	if defaultPC != "" {
		err := json.Unmarshal([]byte(defaultPC), &pc)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal default fleet cluster agent priority class: %w", err)
		}
	} else {
		pc = nil
	}

	defaultPdb := settings.FleetAgentDefaultPodDisruptionBudget.Get()
	pdb := new(TT)
	if defaultPdb != "" {
		err := json.Unmarshal([]byte(defaultPdb), &pdb)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal default fleet cluster agent priority class: %w", err)
		}
	} else {
		pdb = nil
	}

	return pc, pdb, nil
}
