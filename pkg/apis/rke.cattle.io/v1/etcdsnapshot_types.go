package v1

import (
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SnapshotMetadataClusterSpecKey = "provisioning-cluster-spec"

	RestoreRKEConfigNone              = "none"
	RestoreRKEConfigKubernetesVersion = "kubernetesVersion"
	RestoreRKEConfigAll               = "all"
)

// ETCDSnapshotS3 defines S3 snapshot configuration for ETCD backups.
type ETCDSnapshotS3 = mgmt.ETCDSnapshotS3

// +genclient
// +kubebuilder:resource:path=etcdsnapshots,scope=Namespaced
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}

// ETCDSnapshot is the top-level resource representing a snapshot operation.
type ETCDSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ETCDSnapshot.
	// +optional
	Spec ETCDSnapshotSpec `json:"spec,omitempty"`

	// SnapshotFile holds metadata about the snapshot file produced by this snapshot operation.
	// +optional
	SnapshotFile ETCDSnapshotFile `json:"snapshotFile,omitempty"`

	// Status contains information about the current state of the snapshot operation.
	// +optional
	Status ETCDSnapshotStatus `json:"status,omitempty"`
}

// ETCDSnapshotSpec defines the desired state of a snapshot.
type ETCDSnapshotSpec struct {
	// ClusterName is the name of the cluster (cluster.provisioning.cattle.io) for which this snapshot was taken.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
}

// ETCDSnapshotFile holds metadata about a snapshot file.
type ETCDSnapshotFile struct {
	// Name is the full snapshot name. It consists of the cluster name prefix,
	// followed by the base snapshot identifier and ends with an optional storage suffix (e.g. "s3").
	// The typical format is:
	//   <cluster>-etcd-snapshot-<cluster>-<nodepool>-<uniqueid>-<timestamp>[-<storage-type>]
	// The base snapshot identifier follows:
	//   etcd-snapshot-<cluster>-<nodepool>-<uniqueid>-<timestamp>
	// +optional
	Name string `json:"name,omitempty"`

	// NodeName is the name of the downstream node where the snapshot was created.
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Location is the absolute file:// or s3:// URI address of the snapshot.
	// +optional
	Location string `json:"location,omitempty"`

	// Metadata contains a base64-encoded, gzipped snapshot of the cluster spec at the time the snapshot was taken.
	// +optional
	Metadata string `json:"metadata,omitempty"`

	// CreatedAt is the timestamp when the snapshot was created.
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// Size is the size of the snapshot file in bytes.
	// +optional
	Size int64 `json:"size,omitempty"`

	// S3 holds metadata about the S3 destination if the snapshot is stored remotely. If nil, the snapshot
	// is assumed to be stored locally and associated with the owning CAPI machine.
	// +optional
	S3 *ETCDSnapshotS3 `json:"s3,omitempty"`

	// Status represents the current state of the snapshot, such as "successful" or "failed".
	// +optional
	Status string `json:"status,omitempty"`

	// Message is a string detailing the encountered error during snapshot creation if specified.
	// +optional
	Message string `json:"message,omitempty"`
}

// ETCDSnapshotStatus describes the observed state of the snapshot.
type ETCDSnapshotStatus struct {
	// This field is currently unused but retained for backward compatibility or future use.
	Missing bool `json:"missing"`
}
