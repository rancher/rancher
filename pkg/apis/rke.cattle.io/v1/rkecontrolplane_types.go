package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RKEControlPlaneSpec struct {
	ClusterConfiguration `json:",inline"`

	// AgentEnvVars is a list of environment variables that will be set on
	// the cluster agent deployment and system agent service.
	// +nullable
	// +optional
	AgentEnvVars []EnvVar `json:"agentEnvVars,omitempty"`

	// LocalClusterAuthEndpoint is the configuration for the local cluster
	// auth endpoint.
	// +optional
	LocalClusterAuthEndpoint LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`

	// ETCDSnapshotCreate is the configuration for the etcd snapshot creation
	// operation.
	// +nullable
	// +optional
	ETCDSnapshotCreate *ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`

	// ETCDSnapshotRestore is the configuration for the etcd snapshot restore
	// operation.
	// +nullable
	// +optional
	ETCDSnapshotRestore *ETCDSnapshotRestore `json:"etcdSnapshotRestore,omitempty"`

	// RotateCertificates is the configuration for the certificate rotation
	// operation.
	// +nullable
	// +optional
	RotateCertificates *RotateCertificates `json:"rotateCertificates,omitempty"`

	// RotateEncryptionKeys is the configuration for the encryption key
	// rotation operation.
	// +nullable
	// +optional
	RotateEncryptionKeys *RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	// KubernetesVersion is the desired version of RKE2/K3s for the cluster.
	// This field is only populated for provisioned and custom clusters.
	// +nullable
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// ClusterName is the name of the provisioning cluster object.
	// +kubebuilder:validation:MaxLength=63
	// +required
	ClusterName string `json:"clusterName,omitempty"`

	// ManagementClusterName is the name of the management cluster object
	// that relates to this cluster.
	// +required
	ManagementClusterName string `json:"managementClusterName,omitempty"`

	// UnmanagedConfig indicates whether the configuration files for this
	// cluster are managed by Rancher or externally.
	UnmanagedConfig bool `json:"unmanagedConfig,omitempty"`
}

type RKEControlPlaneStatus struct {
	// AppliedSpec is the state for which the last reconciliation loop for
	// the controlplane was completed.
	// +optional
	AppliedSpec *RKEControlPlaneSpec `json:"appliedSpec,omitempty"`

	// Conditions is a representation of the current state of the
	// RKEControlPlane object.
	// This includes its machine reconciliation status
	// (Bootstrapped, Provisioned, Stable, Reconciled), the status of the
	// system-upgrade-controller (SystemUpgradeControllerReady), and CAPI
	// required conditions (ScalingUp, ScalingDown, RollingOut).
	// Information related to errors encountered while transitioning to one
	// of these states will be populated in the Message and Reason fields.
	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// Ready denotes that the API server has been initialized and is ready to
	// receive requests.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// ObservedGeneration is the generation for which the RKEControlPlane has
	// started processing.
	ObservedGeneration int64 `json:"observedGeneration"`

	// CertificateRotationGeneration is the last observed state for which the
	// certificate rotation operation was successful.
	// +optional
	CertificateRotationGeneration int64 `json:"certificateRotationGeneration,omitempty"`

	// RotateEncryptionKeys is the state for which the last encryption key
	// rotation operation was successful.
	// +optional
	RotateEncryptionKeys *RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	// RotateEncryptionKeysPhase is the phase the encryption key
	// rotation operation is currently executing.
	// +optional
	RotateEncryptionKeysPhase RotateEncryptionKeysPhase `json:"rotateEncryptionKeysPhase,omitempty"`

	// RotateEncryptionKeysLeader is the name of the CAPI machine object
	// which has been elected leader of the controlplane nodes for
	// encryption key rotation purposes.
	// +optional
	RotateEncryptionKeysLeader string `json:"rotateEncryptionKeysLeader,omitempty"`

	// ETCDSnapshotRestore is the state for which the last etcd snapshot
	// restore operation was successful.
	// +optional
	ETCDSnapshotRestore *ETCDSnapshotRestore `json:"etcdSnapshotRestore,omitempty"`

	// ETCDSnapshotRestorePhase is the phase the etcd snapshot
	// restore operation is currently executing.
	// +kubebuilder:validation:Enum=Started;Shutdown;Restore;PostRestorePodCleanup;InitialRestartCluster;PostRestoreNodeCleanup;RestartCluster;Finished;Failed
	// +optional
	ETCDSnapshotRestorePhase ETCDSnapshotPhase `json:"etcdSnapshotRestorePhase,omitempty"`

	// ETCDSnapshotCreate is the state for which the last etcd snapshot
	// create operation was successful.
	// +optional
	ETCDSnapshotCreate *ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`

	// ETCDSnapshotCreatePhase is the phase the etcd snapshot create
	// operation is currently executing.
	// +kubebuilder:validation:Enum=Started;RestartCluster;Finished;Failed
	// +optional
	ETCDSnapshotCreatePhase ETCDSnapshotPhase `json:"etcdSnapshotCreatePhase,omitempty"`

	// ConfigGeneration is the current generation of the configuration for a
	// given cluster.
	// Changing this value (which is done automatically during an etcd restore) will trigger a reconciliation loop
	// which will invoke draining (if enabled).
	// +optional
	ConfigGeneration int64 `json:"configGeneration,omitempty"`

	// Initialized denotes that the API server is initialized and worker
	// nodes can be joined to the cluster.
	// +optional
	Initialized bool `json:"initialized,omitempty"`

	// AgentConnected denotes that the cluster-agent connection is currently
	// established for the cluster.
	// +optional
	AgentConnected bool `json:"agentConnected,omitempty"`
}

// +genclient
// +kubebuilder:resource:path=rkecontrolplanes,shortName=rcp,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"cluster.x-k8s.io/v1beta1=v1","auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Initialized",type=string,JSONPath=".status.initialized",description="This denotes whether or not the control plane is initialized"
// +kubebuilder:printcolumn:name="API Server Available",type=boolean,JSONPath=".status.ready",description="RKEControlPlane API Server is ready to receive requests"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp",description="Time duration since creation of RKEControlPlane"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.kubernetesVersion",description="Kubernetes version associated with this control plane"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RKEControlPlane is the Schema for the controlplane.
type RKEControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the controlplane.
	// +optional
	Spec RKEControlPlaneSpec `json:"spec,omitempty"`

	// Status is the observed state of the controlplane.
	// +optional
	Status RKEControlPlaneStatus `json:"status,omitempty"`
}
