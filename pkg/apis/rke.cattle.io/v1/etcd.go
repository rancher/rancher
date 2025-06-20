package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ETCDSnapshotPhase string

const (
	ETCDSnapshotPhaseStarted                ETCDSnapshotPhase = "Started"
	ETCDSnapshotPhaseShutdown               ETCDSnapshotPhase = "Shutdown"
	ETCDSnapshotPhaseRestore                ETCDSnapshotPhase = "Restore"
	ETCDSnapshotPhasePostRestorePodCleanup  ETCDSnapshotPhase = "PostRestorePodCleanup"
	ETCDSnapshotPhaseInitialRestartCluster  ETCDSnapshotPhase = "InitialRestartCluster"
	ETCDSnapshotPhasePostRestoreNodeCleanup ETCDSnapshotPhase = "PostRestoreNodeCleanup"
	ETCDSnapshotPhaseRestartCluster         ETCDSnapshotPhase = "RestartCluster"
	ETCDSnapshotPhaseFinished               ETCDSnapshotPhase = "Finished"
	ETCDSnapshotPhaseFailed                 ETCDSnapshotPhase = "Failed"
)

type ETCDSnapshotS3 struct {
	// Endpoint is the S3 endpoint used for snapshot operations.
	Endpoint string `json:"endpoint,omitempty"`
	// EndpointCA is the CA certificate for validating the S3 endpoint.
	EndpointCA string `json:"endpointCA,omitempty"`
	// SkipSSLVerify is a flag used to ignore certificate validation errors when using the configured endpoint.
	SkipSSLVerify bool `json:"skipSSLVerify,omitempty"`
	// Bucket is the name of the S3 bucket used for snapshot operations.
	Bucket string `json:"bucket,omitempty"`
	// Region is the S3 region used for snapshot operations.
	Region string `json:"region,omitempty"`
	// CloudCredentialName is the name of the secret containing the credentials used to access the S3 bucket.
	// The secret is expected to have the following keys:
	// - accessKey [required]
	// - secretKey [required]
	// - defaultRegion
	// - defaultEndpoint
	// - defaultEndpointCA
	// - defaultSkipSSLVerify
	// - defaultBucket
	// - defaultFolder
	CloudCredentialName string `json:"cloudCredentialName,omitempty"`
	// Folder is the name of the S3 folder used for snapshot operations.
	Folder string `json:"folder,omitempty"`
}

type ETCDSnapshotCreate struct {
	// Changing the Generation is the only thing required to initiate a snapshot creation.
	Generation int `json:"generation,omitempty"`
}

type ETCDSnapshotRestore struct {
	// Name refers to the name of the associated etcdsnapshot object.
	Name string `json:"name,omitempty"`

	// Changing the Generation is the only thing required to initiate a snapshot restore.
	// +optional
	Generation int `json:"generation,omitempty"`
	// Set to either none (or empty string), all, or kubernetesVersion
	// +kubebuilder:validation:Enum=none;all;kubernetesVersion
	// +optional
	RestoreRKEConfig string `json:"restoreRKEConfig,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ETCDSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ETCDSnapshotSpec   `json:"spec,omitempty"`
	SnapshotFile      ETCDSnapshotFile   `json:"snapshotFile,omitempty"`
	Status            ETCDSnapshotStatus `json:"status"`
}

type ETCDSnapshotSpec struct {
	ClusterName string `json:"clusterName,omitempty"`
}

type ETCDSnapshotFile struct {
	Name      string          `json:"name,omitempty"`
	NodeName  string          `json:"nodeName,omitempty"`
	Location  string          `json:"location,omitempty"`
	Metadata  string          `json:"metadata,omitempty"`
	CreatedAt *metav1.Time    `json:"createdAt,omitempty"`
	Size      int64           `json:"size,omitempty"`
	S3        *ETCDSnapshotS3 `json:"s3,omitempty"`
	Status    string          `json:"status,omitempty"`
	Message   string          `json:"message,omitempty"`
}

type ETCDSnapshotStatus struct {
	Missing bool `json:"missing"`
}

type ETCD struct {
	// DisableSnapshots disables the creation of snapshots for the cluster.
	// +optional
	DisableSnapshots bool `json:"disableSnapshots,omitempty"`
	// SnapshotScheduleCron is the cron schedule for the snapshot creation.
	// +optional
	SnapshotScheduleCron string `json:"snapshotScheduleCron,omitempty"`
	// SnapshotRetention is the number of snapshots the downstream cluster should retain per snapshot generation.
	// +optional
	SnapshotRetention int `json:"snapshotRetention,omitempty"`
	// S3 defines the S3 configuration for the cluster if enabled.
	// +optional
	S3 *ETCDSnapshotS3 `json:"s3,omitempty"`
}
