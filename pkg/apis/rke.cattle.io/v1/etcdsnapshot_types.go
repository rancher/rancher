package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ETCDSnapshotS3 defines S3 snapshot configuration for ETCD backups.
type ETCDSnapshotS3 struct {
	// Endpoint is the S3 endpoint used for snapshot operations.
	// If this field is not explicitly set, the 'defaultEndpoint' value from the referenced CloudCredential will be used.
	// +nullable
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// EndpointCA is the CA certificate for validating the S3 endpoint.
	// This can be either a file path (e.g., "/etc/ssl/certs/my-ca.crt")
	// or the CA certificate content, in base64-encoded or plain PEM format.
	// If this field is not explicitly set, the 'defaultEndpointCA' value from the referenced CloudCredential will be used.
	// +nullable
	// +optional
	EndpointCA string `json:"endpointCA,omitempty"`

	// SkipSSLVerify defines whether TLS certificate verification is disabled.
	// If this field is not explicitly set, the 'defaultSkipSSLVerify' value
	// from the referenced CloudCredential will be used.
	// +optional
	SkipSSLVerify bool `json:"skipSSLVerify,omitempty"`

	// Bucket is the name of the S3 bucket used for snapshot operations.
	// If this field is not explicitly set, the 'defaultBucket' value from the referenced CloudCredential will be used.
	// An empty bucket name will cause a 'failed to initialize S3 client: s3 bucket name was not set' error.
	// +kubebuilder:validation:MaxLength=63
	// +nullable
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Region is the S3 region used for snapshot operations. (e.g., "us-east-1").
	// If this field is not explicitly set, the 'defaultRegion' value from the referenced CloudCredential will be used.
	// +nullable
	// +optional
	Region string `json:"region,omitempty"`

	// CloudCredentialName is the name of the secret containing the
	// credentials used to access the S3 bucket.
	// The secret is expected to have the following keys:
	// - accessKey [required]
	// - secretKey [required]
	// - defaultRegion
	// - defaultEndpoint
	// - defaultEndpointCA
	// - defaultSkipSSLVerify
	// - defaultBucket
	// - defaultFolder
	// Fields set directly in this spec (`ETCDSnapshotS3`) take precedence over the corresponding
	// values from the CloudCredential secret. This field must be in the format of "namespace:name".
	// +nullable
	// +optional
	CloudCredentialName string `json:"cloudCredentialName,omitempty"`

	// Folder is the name of the S3 folder used for snapshot operations.
	// If this field is not explicitly set, the folder from the referenced CloudCredential will be used.
	// +nullable
	// +optional
	Folder string `json:"folder,omitempty"`
}

// +genclient
// +kubebuilder:resource:path=etcdsnapshots,scope=Namespaced
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
