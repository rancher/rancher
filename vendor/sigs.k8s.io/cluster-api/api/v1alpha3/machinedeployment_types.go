/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type MachineDeploymentStrategyType string

const (
	// Replace the old MachineSet by new one using rolling update
	// i.e. gradually scale down the old MachineSet and scale up the new one.
	RollingUpdateMachineDeploymentStrategyType MachineDeploymentStrategyType = "RollingUpdate"

	// RevisionAnnotation is the revision annotation of a machine deployment's machine sets which records its rollout sequence
	RevisionAnnotation = "machinedeployment.clusters.x-k8s.io/revision"
	// RevisionHistoryAnnotation maintains the history of all old revisions that a machine set has served for a machine deployment.
	RevisionHistoryAnnotation = "machinedeployment.clusters.x-k8s.io/revision-history"
	// DesiredReplicasAnnotation is the desired replicas for a machine deployment recorded as an annotation
	// in its machine sets. Helps in separating scaling events from the rollout process and for
	// determining if the new machine set for a deployment is really saturated.
	DesiredReplicasAnnotation = "machinedeployment.clusters.x-k8s.io/desired-replicas"
	// MaxReplicasAnnotation is the maximum replicas a deployment can have at a given point, which
	// is machinedeployment.spec.replicas + maxSurge. Used by the underlying machine sets to estimate their
	// proportions in case the deployment has surge replicas.
	MaxReplicasAnnotation = "machinedeployment.clusters.x-k8s.io/max-replicas"
)

// ANCHOR: MachineDeploymentSpec

// MachineDeploymentSpec defines the desired state of MachineDeployment
type MachineDeploymentSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// Number of desired machines. Defaults to 1.
	// This is a pointer to distinguish between explicit zero and not specified.
	Replicas *int32 `json:"replicas,omitempty"`

	// Label selector for machines. Existing MachineSets whose machines are
	// selected by this will be the ones affected by this deployment.
	// It must match the machine template's labels.
	Selector metav1.LabelSelector `json:"selector"`

	// Template describes the machines that will be created.
	Template MachineTemplateSpec `json:"template"`

	// The deployment strategy to use to replace existing machines with
	// new ones.
	// +optional
	Strategy *MachineDeploymentStrategy `json:"strategy,omitempty"`

	// Minimum number of seconds for which a newly created machine should
	// be ready.
	// Defaults to 0 (machine will be considered available as soon as it
	// is ready)
	// +optional
	MinReadySeconds *int32 `json:"minReadySeconds,omitempty"`

	// The number of old MachineSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 1.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Indicates that the deployment is paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// The maximum time in seconds for a deployment to make progress before it
	// is considered to be failed. The deployment controller will continue to
	// process failed deployments and a condition with a ProgressDeadlineExceeded
	// reason will be surfaced in the deployment status. Note that progress will
	// not be estimated during the time a deployment is paused. Defaults to 600s.
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// ANCHOR_END: MachineDeploymentSpec

// ANCHOR: MachineDeploymentStrategy

// MachineDeploymentStrategy describes how to replace existing machines
// with new ones.
type MachineDeploymentStrategy struct {
	// Type of deployment. Currently the only supported strategy is
	// "RollingUpdate".
	// Default is RollingUpdate.
	// +optional
	Type MachineDeploymentStrategyType `json:"type,omitempty"`

	// Rolling update config params. Present only if
	// MachineDeploymentStrategyType = RollingUpdate.
	// +optional
	RollingUpdate *MachineRollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

// ANCHOR_END: MachineDeploymentStrategy

// ANCHOR: MachineRollingUpdateDeployment

// MachineRollingUpdateDeployment is used to control the desired behavior of rolling update.
type MachineRollingUpdateDeployment struct {
	// The maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The maximum number of machines that can be scheduled above the
	// desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of
	// desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// ANCHOR_END: MachineRollingUpdateDeployment

// ANCHOR: MachineDeploymentStatus

// MachineDeploymentStatus defines the observed state of MachineDeployment
type MachineDeploymentStatus struct {
	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Selector is the same as the label selector but in the string format to avoid introspection
	// by clients. The string will be in the same format as the query-param syntax.
	// More info about label selectors: http://kubernetes.io/docs/user-guide/labels#label-selectors
	// +optional
	Selector string `json:"selector,omitempty"`

	// Total number of non-terminated machines targeted by this deployment
	// (their labels match the selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of non-terminated machines targeted by this deployment
	// that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Total number of ready machines targeted by this deployment.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Total number of available machines (ready for at least minReadySeconds)
	// targeted by this deployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Total number of unavailable machines targeted by this deployment.
	// This is the total number of machines that are still required for
	// the deployment to have 100% available capacity. They may either
	// be machines that are running but not yet available or machines
	// that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// Phase represents the current phase of a MachineDeployment (ScalingUp, ScalingDown, Running, Failed, or Unknown).
	// +optional
	Phase string `json:"phase,omitempty"`
}

// ANCHOR_END: MachineDeploymentStatus

// MachineDeploymentPhase indicates the progress of the machine deployment
type MachineDeploymentPhase string

const (
	// MachineDeploymentPhaseScalingUp indicates the MachineDeployment is scaling up.
	MachineDeploymentPhaseScalingUp = MachineDeploymentPhase("ScalingUp")

	// MachineDeploymentPhaseScalingDown indicates the MachineDeployment is scaling down.
	MachineDeploymentPhaseScalingDown = MachineDeploymentPhase("ScalingDown")

	// MachineDeploymentPhaseRunning indicates scaling has completed and all Machines are running.
	MachineDeploymentPhaseRunning = MachineDeploymentPhase("Running")

	// MachineDeploymentPhaseFailed indicates there was a problem scaling and user intervention might be required.
	MachineDeploymentPhaseFailed = MachineDeploymentPhase("Failed")

	// MachineDeploymentPhaseUnknown indicates the state of the MachineDeployment cannot be determined.
	MachineDeploymentPhaseUnknown = MachineDeploymentPhase("Unknown")
)

// SetTypedPhase sets the Phase field to the string representation of MachineDeploymentPhase.
func (md *MachineDeploymentStatus) SetTypedPhase(p MachineDeploymentPhase) {
	md.Phase = string(p)
}

// GetTypedPhase attempts to parse the Phase field and return
// the typed MachineDeploymentPhase representation.
func (md *MachineDeploymentStatus) GetTypedPhase() MachineDeploymentPhase {
	switch phase := MachineDeploymentPhase(md.Phase); phase {
	case
		MachineDeploymentPhaseScalingDown,
		MachineDeploymentPhaseScalingUp,
		MachineDeploymentPhaseRunning,
		MachineDeploymentPhaseFailed:
		return phase
	default:
		return MachineDeploymentPhaseUnknown
	}
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machinedeployments,shortName=md,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="MachineDeployment status such as ScalingUp/ScalingDown/Running/Failed/Unknown"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas",description="Total number of non-terminated machines targeted by this deployment"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas",description="Total number of available machines (ready for at least minReadySeconds)"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Total number of ready machines targeted by this deployment."

// MachineDeployment is the Schema for the machinedeployments API
type MachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineDeploymentSpec   `json:"spec,omitempty"`
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MachineDeploymentList contains a list of MachineDeployment
type MachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineDeployment{}, &MachineDeploymentList{})
}
