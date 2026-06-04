package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ETCDSnapshotSaveArgs contains parameters for saving an ETCD snapshot.
// Name specifies the name of the snapshot file.
// ETCDSnapshotCompress determines if the snapshot will be compressed.
// ETCDSnapshotDir specifies the directory where the snapshot will be saved.
type ETCDSnapshotSaveArgs struct {
	// Name specifies the name of the ETCD snapshot file.
	// +optional
	Name string `json:"name,omitempty"`

	// ETCDSnapshotCompress determines if the snapshot will be compressed.
	// +optional
	ETCDSnapshotCompress bool `json:"etcd_snapshot_compress,omitempty"`

	// ETCDSnapshotDir specifies the directory where the snapshot will be saved.
	// +optional
	ETCDSnapshotDir string `json:"etcd_snapshot_dir,omitempty"`
}

// ETCDSnapshotSaveSpec defines the desired state of ETCDSnapshotSave.
type ETCDSnapshotSaveSpec struct {
	// +optional
	OperationSpec `json:",inline"`

	// +optional
	Args ETCDSnapshotSaveArgs `json:"args,omitempty"`
}

// ETCDSnapshotSaveStep is the step of the ETCDSnapshotSave operation.
type ETCDSnapshotSaveStep string

const (
	// ETCDSnapshotSaveStepSave indicates the step is to save the snapshot.
	ETCDSnapshotSaveStepSave ETCDSnapshotSaveStep = "Save"

	// ETCDSnapshotSaveStepRestart indicates the step is to restart the cluster.
	ETCDSnapshotSaveStepRestart ETCDSnapshotSaveStep = "Restart"
)

// ETCDSnapshotSaveStatus defines the observed state of ETCDSnapshotSave.
type ETCDSnapshotSaveStatus struct {
	// Operation status is the shared status common to all operations.
	OperationStatus `json:",inline"`

	// Step is the current step of the operation.
	// Step is typically only valid during the InProgress phase.
	// +kubebuilder:validation:Enum=Save;Restart
	// +optional
	Step ETCDSnapshotSaveStep `json:"step,omitempty"`
}

func (s *ETCDSnapshotSaveStatus) SetPhase(phase OperationPhase) {
	if s.Phase == phase {
		return
	}
	s.Phase = phase
	s.LastUpdated = metav1.Now()
}

func (s *ETCDSnapshotSaveStatus) SetStep(step ETCDSnapshotSaveStep) {
	if s.Step == step {
		return
	}
	s.Step = step
	s.LastUpdated = metav1.Now()
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=etcdsnapshotsaves,scope=Namespaced,categories=operations
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.clusterRef.Name"
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=".spec.paused"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=".status.step"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// ETCDSnapshotSave is the mechanism for initiating an RKE2 or K3s etcd-snapshot save operation for v2prov, CAPI, and
// imported clusters.
type ETCDSnapshotSave struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ETCDSnapshotSave.
	// +required
	Spec ETCDSnapshotSaveSpec `json:"spec,omitempty"`

	// Status is the observed state of the ETCDSnapshotSave.
	// +optional
	Status ETCDSnapshotSaveStatus `json:"status,omitempty"`
}
