package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ETCDSnapshotS3 struct {
	// Endpoint is the S3 endpoint used for snapshot operations.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// EndpointCA is the CA certificate for validating the S3 endpoint.
	// +optional
	EndpointCA string `json:"endpointCA,omitempty"`

	// SkipSSLVerify is a flag used to ignore certificate validation errors
	// when using the configured endpoint.
	// +optional
	SkipSSLVerify bool `json:"skipSSLVerify,omitempty"`

	// Bucket is the name of the S3 bucket used for snapshot operations.
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Region is the S3 region used for snapshot operations.
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
	// +optional
	CloudCredentialName string `json:"cloudCredentialName,omitempty"`

	// Folder is the name of the S3 folder used for snapshot operations.
	// +optional
	Folder string `json:"folder,omitempty"`
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
