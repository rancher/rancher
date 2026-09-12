package v3

// K3sConfig defines the configuration options for a K3s cluster.
type K3sConfig struct {
	// Version specifies the Kubernetes version for the K3s cluster configuration.
	Version string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`

	// ETCD defines the etcd configuration settings for 3s clusters, including snapshot retention and S3.
	ETCD ETCD `yaml:"etcd" json:"etcd,omitempty"`

	// ClusterUpgradeStrategy defines the upgrade strategy with concurrency and node draining options for K3s clusters.
	ClusterUpgradeStrategy `yaml:"k3s_upgrade_strategy,omitempty" json:"k3supgradeStrategy,omitempty"`
}

// Rke2Config defines the configuration options for an RKE2 cluster.
type Rke2Config struct {
	// Version specifies the Kubernetes version for the RKE2 cluster configuration.
	Version string `yaml:"kubernetes_version" json:"kubernetesVersion,omitempty"`

	// ETCD defines the etcd configuration settings for RKE2 clusters, including snapshot retention and S3.
	ETCD ETCD `yaml:"etcd" json:"etcd,omitempty"`

	// ClusterUpgradeStrategy defines the upgrade strategy with concurrency and node draining options for RKE2 clusters.
	ClusterUpgradeStrategy `yaml:"rke2_upgrade_strategy,omitempty" json:"rke2upgradeStrategy,omitempty"`
}

// ETCD defines the etcd configuration for RKE2/K3s clusters
type ETCD struct {
	// DisableSnapshots disables the creation of snapshots for the cluster.
	// +optional
	DisableSnapshots bool `json:"disableSnapshots,omitempty"`

	// SnapshotScheduleCron is the cron schedule for the snapshot creation.
	// +nullable
	// +optional
	SnapshotScheduleCron string `json:"snapshotScheduleCron,omitempty"`

	// SnapshotRetention is the number of snapshots the downstream cluster
	// should retain per snapshot generation.
	// +optional
	SnapshotRetention int `json:"snapshotRetention,omitempty"`

	// S3 defines the S3 configuration for the cluster if enabled.
	// +nullable
	// +optional
	S3 *ETCDSnapshotS3 `json:"s3,omitempty"`
}

// ETCDSnapshotS3 defines S3 snapshot configuration for etcd backups.
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

	// Retention defines the number of snapshots to retain in the S3 bucket.
	// Older snapshots beyond this retention count will be deleted.
	// If this field is not explicitly set, the retention value from the etcd retention will be used.
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=0
	Retention int `json:"retention,omitempty"`

	// Folder is the name of the S3 folder used for snapshot operations.
	// If this field is not explicitly set, the folder from the referenced CloudCredential will be used.
	// +nullable
	// +optional
	Folder string `json:"folder,omitempty"`
}

// ClusterUpgradeStrategy provides configuration to the downstream system-upgrade-controller
type ClusterUpgradeStrategy struct {
	// How many controlplane nodes should be upgrade at time, defaults to 1
	ServerConcurrency int `yaml:"server_concurrency" json:"serverConcurrency,omitempty" norman:"min=1"`
	// How many workers should be upgraded at a time
	WorkerConcurrency int `yaml:"worker_concurrency" json:"workerConcurrency,omitempty" norman:"min=1"`
	// Whether controlplane nodes should be drained
	DrainServerNodes bool `yaml:"drain_server_nodes" json:"drainServerNodes,omitempty"`
	// Whether worker nodes should be drained
	DrainWorkerNodes bool `yaml:"drain_worker_nodes" json:"drainWorkerNodes,omitempty"`
}

func (r *Rke2Config) SetStrategy(serverConcurrency, workerConcurrency int) {
	r.ClusterUpgradeStrategy.ServerConcurrency = serverConcurrency
	r.ClusterUpgradeStrategy.WorkerConcurrency = workerConcurrency
}
func (k *K3sConfig) SetStrategy(serverConcurrency, workerConcurrency int) {
	k.ClusterUpgradeStrategy.ServerConcurrency = serverConcurrency
	k.ClusterUpgradeStrategy.WorkerConcurrency = workerConcurrency
}
