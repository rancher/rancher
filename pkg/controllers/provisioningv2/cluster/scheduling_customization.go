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

// updateV1SchedulingCustomization looks for the provisioning.cattle.io/enable-{fleet-}scheduling-customization
// annotation on the v1.Cluster and if found populates the
// spec.{Cluster,Fleet}AgentDeploymentCustomization.SchedulingCustomization fields with the default values set in the
// global settings. If the cluster-agent-scheduling-customization feature is disabled, the cluster will be returned
// unchanged. The provisioning.cattle.io/enable-{fleet-}scheduling-customization annotation can be set to 'true' or
// 'false', which will add or remove the scheduling customization field as needed.
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

// updateV3SchedulingCustomization looks for the provisioning.cattle.io/enable-{fleet-}scheduling-customization
// annotation on the v3.Cluster and if found populates the
// spec.{Cluster,Fleet}AgentDeploymentCustomization.SchedulingCustomization fields with the default values set in the
// global settings. If the cluster-agent-scheduling-customization feature is disabled, the cluster will be returned
// unchanged. The provisioning.cattle.io/enable-{fleet-}scheduling-customization annotation can be set to 'true' or
// 'false', which will add or remove the scheduling customization field as needed. updateV3SchedulingCustomization is
// intended to handle KEv2 and legacy clusters specifically.
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

// updateV1AgentSchedulingCustomization looks for the provisioning.cattle.io/enable-scheduling-customization annotation
// on the v1.Cluster and if found populates the spec.ClusterAgentDeploymentCustomization.SchedulingCustomization fields
// with the default values set in the global settings. The provisioning.cattle.io/enable-scheduling-customization
// annotation can be set to 'true' or 'false', which will add or remove the scheduling customization field as needed.
func (h *handler) updateV1AgentSchedulingCustomization(cluster *v1.Cluster) (*v1.Cluster, error) {
	return h.updateV1SchedulingCustomizationForAgent(cluster, clusterAgent)
}

// updateV1FleetAgentSchedulingCustomization looks for the provisioning.cattle.io/enable-fleet-scheduling-customization
// annotation on the v1.Cluster and if found populates the spec.FleetAgentDeploymentCustomization.SchedulingCustomization fields
// with the default values set in the global settings. The provisioning.cattle.io/enable-fleet-scheduling-customization
// annotation can be set to 'true' or 'false', which will add or remove the scheduling customization field as needed.
func (h *handler) updateV1FleetAgentSchedulingCustomization(cluster *v1.Cluster) (*v1.Cluster, error) {
	return h.updateV1SchedulingCustomizationForAgent(cluster, fleetAgent)
}

type agentType int

const (
	clusterAgent agentType = iota
	fleetAgent
)

func (h *handler) updateV1SchedulingCustomizationForAgent(cluster *v1.Cluster, agent agentType) (*v1.Cluster, error) {
	var annotation string
	var pcSetting, pdbSetting settings.Setting

	switch agent {
	case clusterAgent:
		annotation = manageSchedulingDefaultsAnn
		pcSetting = settings.ClusterAgentDefaultPriorityClass
		pdbSetting = settings.ClusterAgentDefaultPodDisruptionBudget
	case fleetAgent:
		annotation = manageFleetSchedulingDefaultsAnn
		pcSetting = settings.FleetAgentDefaultPriorityClass
		pdbSetting = settings.FleetAgentDefaultPodDisruptionBudget
	default:
		return nil, fmt.Errorf("unknown agent type: %v", agent)
	}

	value, ok := cluster.Annotations[annotation]
	if !ok {
		return cluster, nil
	}

	lowerVal := strings.ToLower(value)
	if lowerVal != "true" && lowerVal != "false" {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()

	var adc *v1.AgentDeploymentCustomization
	switch agent {
	case clusterAgent:
		adc = cluster.Spec.ClusterAgentDeploymentCustomization
	case fleetAgent:
		adc = cluster.Spec.FleetAgentDeploymentCustomization
	default:
		return nil, fmt.Errorf("unknown agent type during adc assignment: %v", agent)
	}

	if lowerVal == "false" {
		delete(cluster.Annotations, annotation)

		if adc != nil {
			adc.SchedulingCustomization = nil
		}

		return h.clusters.Update(cluster)
	}

	// annotation was added to a cluster that already has the fields set, we should not override the existing values.
	if adc != nil && adc.SchedulingCustomization != nil {
		delete(cluster.Annotations, annotation)
		return h.clusters.Update(cluster)
	}

	defaultPC, defaultPDB, err := getDefaultSchedulingCustomization[v1.PriorityClassSpec, v1.PodDisruptionBudgetSpec](
		pcSetting,
		pdbSetting,
	)
	if err != nil {
		return cluster, fmt.Errorf("failed to get default scheduling customization: %w", err)
	}
	if defaultPDB != nil || defaultPC != nil {
		if adc == nil {
			adc = &v1.AgentDeploymentCustomization{}
			switch agent {
			case clusterAgent:
				cluster.Spec.ClusterAgentDeploymentCustomization = adc
			case fleetAgent:
				cluster.Spec.FleetAgentDeploymentCustomization = adc
			default:
				return nil, fmt.Errorf("unknown agent type during adc assignment: %v", agent)
			}
		}
		adc.SchedulingCustomization = &v1.AgentSchedulingCustomization{
			PodDisruptionBudget: defaultPDB,
			PriorityClass:       defaultPC,
		}
	}

	delete(cluster.Annotations, annotation)
	return h.clusters.Update(cluster)
}

// updateV3AgentSchedulingCustomization looks for the provisioning.cattle.io/enable-scheduling-customization annotation
// on the v3.Cluster and if found populates the spec.ClusterAgentDeploymentCustomization.SchedulingCustomization fields
// with the default values set in the global settings. The provisioning.cattle.io/enable-scheduling-customization
// annotation can be set to 'true' or 'false', which will add or remove the scheduling customization field as needed.
// updateV3AgentSchedulingCustomization is intended to handle KEv2 and legacy clusters specifically.
func (h *handler) updateV3AgentSchedulingCustomization(cluster *v3.Cluster) (*v3.Cluster, error) {
	return h.updateV3SchedulingCustomizationForAgent(cluster, clusterAgent)
}

// updateV3FleetAgentSchedulingCustomization looks for the provisioning.cattle.io/enable-fleet-scheduling-customization
// annotation on the v3.Cluster and if found populates the
// spec.FleetAgentDeploymentCustomization.SchedulingCustomization fields with the default values set in the global
// settings. The provisioning.cattle.io/enable-fleet-scheduling-customization annotation can be set to 'true' or
// 'false', which will add or remove the scheduling customization field as needed.
// updateV3FleetAgentSchedulingCustomization is intended to handle KEv2 and legacy clusters specifically.
func (h *handler) updateV3FleetAgentSchedulingCustomization(cluster *v3.Cluster) (*v3.Cluster, error) {
	return h.updateV3SchedulingCustomizationForAgent(cluster, fleetAgent)
}

func (h *handler) updateV3SchedulingCustomizationForAgent(cluster *v3.Cluster, agent agentType) (*v3.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}
	if !h.isLegacyCluster(cluster) {
		return nil, nil
	}

	var annotation string
	var pcSetting, pdbSetting settings.Setting

	switch agent {
	case clusterAgent:
		annotation = manageSchedulingDefaultsAnn
		pcSetting = settings.ClusterAgentDefaultPriorityClass
		pdbSetting = settings.ClusterAgentDefaultPodDisruptionBudget
	case fleetAgent:
		annotation = manageFleetSchedulingDefaultsAnn
		pcSetting = settings.FleetAgentDefaultPriorityClass
		pdbSetting = settings.FleetAgentDefaultPodDisruptionBudget
	default:
		return nil, fmt.Errorf("unknown agent type: %v", agent)
	}

	value, ok := cluster.Annotations[annotation]
	if !ok {
		return cluster, nil
	}

	lowerVal := strings.ToLower(value)
	if lowerVal != "true" && lowerVal != "false" {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()

	var adc *v3.AgentDeploymentCustomization
	switch agent {
	case clusterAgent:
		adc = cluster.Spec.ClusterAgentDeploymentCustomization
	case fleetAgent:
		adc = cluster.Spec.FleetAgentDeploymentCustomization
	default:
		return nil, fmt.Errorf("unknown agent type during adc assignment: %v", agent)
	}

	if lowerVal == "false" {
		delete(cluster.Annotations, annotation)

		if adc != nil {
			adc.SchedulingCustomization = nil
		}

		return h.mgmtClusters.Update(cluster)
	}

	// annotation was added to a cluster that already has the fields set, we should not override the existing values.
	if adc != nil && adc.SchedulingCustomization != nil {
		delete(cluster.Annotations, annotation)
		return h.mgmtClusters.Update(cluster)
	}

	defaultPC, defaultPDB, err := getDefaultSchedulingCustomization[v3.PriorityClassSpec, v3.PodDisruptionBudgetSpec](
		pcSetting,
		pdbSetting,
	)
	if err != nil {
		return cluster, fmt.Errorf("failed to get default scheduling customization: %w", err)
	}
	if defaultPDB != nil || defaultPC != nil {
		if adc == nil {
			adc = &v3.AgentDeploymentCustomization{}
			switch agent {
			case clusterAgent:
				cluster.Spec.ClusterAgentDeploymentCustomization = adc
			case fleetAgent:
				cluster.Spec.FleetAgentDeploymentCustomization = adc
			}
		}
		adc.SchedulingCustomization = &v3.AgentSchedulingCustomization{
			PodDisruptionBudget: defaultPDB,
			PriorityClass:       defaultPC,
		}
	}

	delete(cluster.ObjectMeta.Annotations, annotation)
	return h.mgmtClusters.Update(cluster)
}

func getDefaultSchedulingCustomization[T v1.PriorityClassSpec | v3.PriorityClassSpec, TT v1.PodDisruptionBudgetSpec | v3.PodDisruptionBudgetSpec](
	defaultPriorityClassSetting, defaultPodDisruptionClassSetting settings.Setting,
) (*T, *TT, error) {
	defaultPC := defaultPriorityClassSetting.Get()
	pc := new(T)
	if defaultPC != "" {
		err := json.Unmarshal([]byte(defaultPC), &pc)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal default cluster agent priority class: %w", err)
		}
	} else {
		pc = nil
	}

	defaultPdb := defaultPodDisruptionClassSetting.Get()
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
