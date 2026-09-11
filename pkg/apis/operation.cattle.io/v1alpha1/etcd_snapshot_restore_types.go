package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ETCDSnapshotRestoreArgs contains parameters for restoring an ETCD snapshot.
// Name specifies the name of the snapshot file.
type ETCDSnapshotRestoreArgs struct {
	// Name specifies the name of the ETCD snapshot file.
	// +optional
	Name string `json:"name,omitempty"`

	// RestoreMode names a key in the snapshot's restoreModes metadata, e.g. "kubernetesVersion" or
	// "all". The set of modes a given snapshot offers is published on the snapshot as the
	// etcdsnapshot.rke.io/restore-mode-options annotation. Empty or "none" restores etcd without
	// touching cluster configuration.
	//
	// Deliberately unvalidated: a downstream cluster may declare modes this Rancher does not know
	// about.
	// +kubebuilder:validation:MaxLength=63
	// +optional
	RestoreMode string `json:"restoreMode,omitempty"`
}

// ETCDSnapshotRestoreSpec defines the desired state of ETCDSnapshotRestore.
type ETCDSnapshotRestoreSpec struct {
	// OperationSpec is the shared spec common to all operations.
	// +optional
	OperationSpec `json:",inline"`

	// Args contains parameters for restoring an ETCD snapshot.
	// Mutually exclusive with SnapshotRef.
	// +optional
	Args ETCDSnapshotRestoreArgs `json:"args,omitempty"`
}

// ETCDSnapshotRestoreStep is the step of the ETCDSnapshotRestore operation.
type ETCDSnapshotRestoreStep string

const (
	// ETCDSnapshotRestoreStepPreflight indicates the step is performing preflight checks to determine if the operation will succeed.
	ETCDSnapshotRestoreStepPreflight ETCDSnapshotRestoreStep = "Preflight"

	// ETCDSnapshotRestoreStepRestoreClusterConfig indicates the step is applying the requested
	// restore mode, i.e. writing the cluster configuration captured in the snapshot back onto the
	// object that owns the cluster.
	ETCDSnapshotRestoreStepRestoreClusterConfig ETCDSnapshotRestoreStep = "RestoreClusterConfig"

	// ETCDSnapshotRestoreStepShutdown indicates the step is shutting down the cluster.
	ETCDSnapshotRestoreStepShutdown ETCDSnapshotRestoreStep = "Shutdown"

	// ETCDSnapshotRestoreStepRestore indicates the step where the ETCD snapshot is being restored.
	ETCDSnapshotRestoreStepRestore ETCDSnapshotRestoreStep = "Restore"

	// ETCDSnapshotRestoreStepPostRestorePodCleanup indicates the step is cleaning up the restored pods.
	ETCDSnapshotRestoreStepPostRestorePodCleanup ETCDSnapshotRestoreStep = "PostRestorePodCleanup"

	// ETCDSnapshotRestoreStepInitialRestartCluster indicates the step is restarting the cluster by performing a full reconciliation.
	ETCDSnapshotRestoreStepInitialRestartCluster ETCDSnapshotRestoreStep = "InitialRestartCluster"

	// ETCDSnapshotRestoreStepPostRestoreNodeCleanup indicates the step is removing nodes that are no longer part of the cluster.
	ETCDSnapshotRestoreStepPostRestoreNodeCleanup ETCDSnapshotRestoreStep = "PostRestoreNodeCleanup"

	// ETCDSnapshotRestoreStepRestartCluster indicates the step is restarting the cluster by performing a full reconciliation.
	ETCDSnapshotRestoreStepRestartCluster ETCDSnapshotRestoreStep = "RestartCluster"
)

// ETCDSnapshotRestoreStatus defines the observed state of ETCDSnapshotRestore.
type ETCDSnapshotRestoreStatus struct {
	// Operation status is the shared status common to all operations.
	OperationStatus `json:",inline"`

	// Step is the current step of the operation.
	// Step is typically only valid during the InProgress phase.
	// +kubebuilder:validation:Enum=Preflight;RestoreClusterConfig;Shutdown;Restore;PostRestorePodCleanup;InitialRestartCluster;PostRestoreNodeCleanup;RestartCluster
	// +optional
	Step ETCDSnapshotRestoreStep `json:"step,omitempty"`
}

func (s *ETCDSnapshotRestoreStatus) SetPhase(phase OperationPhase) {
	if s.Phase == phase {
		return
	}
	s.Phase = phase
	s.LastUpdated = metav1.Now()
}

func (s *ETCDSnapshotRestoreStatus) SetStep(step ETCDSnapshotRestoreStep) {
	if s.Step == step {
		return
	}
	s.Step = step
	s.LastUpdated = metav1.Now()
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=etcdsnapshotrestores,scope=Namespaced,categories=operations
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.clusterRef.Name"
// +kubebuilder:printcolumn:name="Snapshot",type=string,JSONPath=".spec.args.name"
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=".spec.args.restoreMode"
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=".spec.paused"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=".spec.step"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// ETCDSnapshotRestore is the mechanism for initiating an RKE2 or K3s etcd restore operation for v2prov, CAPI, and
// imported clusters.
type ETCDSnapshotRestore struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ETCDSnapshotRestore.
	// +required
	Spec ETCDSnapshotRestoreSpec `json:"spec,omitempty"`

	// Status is the observed state of the ETCDSnapshotRestore.
	// +optional
	Status ETCDSnapshotRestoreStatus `json:"status,omitempty"`
}
