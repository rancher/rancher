package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ETCDSnapshotPhase string

const (
	ETCDSnapshotPhaseStarted        ETCDSnapshotPhase = "Started"
	ETCDSnapshotPhaseShutdown       ETCDSnapshotPhase = "Shutdown"
	ETCDSnapshotPhaseRestore        ETCDSnapshotPhase = "Restore"
	ETCDSnapshotPhaseRestartCluster ETCDSnapshotPhase = "RestartCluster"
	ETCDSnapshotPhaseFinished       ETCDSnapshotPhase = "Finished"
	ETCDSnapshotPhaseFailed         ETCDSnapshotPhase = "Failed"
)

type ETCDSnapshotS3 struct {
	Endpoint            string `json:"endpoint,omitempty"`
	EndpointCA          string `json:"endpointCA,omitempty"`
	SkipSSLVerify       bool   `json:"skipSSLVerify,omitempty"`
	Bucket              string `json:"bucket,omitempty"`
	Region              string `json:"region,omitempty"`
	CloudCredentialName string `json:"cloudCredentialName,omitempty"`
	Folder              string `json:"folder,omitempty"`
}

type ETCDSnapshotCreate struct {
	// Changing the Generation is the only thing required to initiate a snapshot creation.
	Generation int `json:"generation,omitempty"`
}

type ETCDSnapshotRestore struct {
	// Name refers to the name of the associated etcdsnapshot object
	Name string `json:"name,omitempty"`

	// Changing the Generation is the only thing required to initiate a snapshot restore.
	Generation int `json:"generation,omitempty"`
	// Set to either none (or empty string), all, or kubernetesVersion
	RestoreRKEConfig string `json:"restoreRKEConfig,omitempty"`
}

// +genclient
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
	DisableSnapshots     bool            `json:"disableSnapshots,omitempty"`
	SnapshotScheduleCron string          `json:"snapshotScheduleCron,omitempty"`
	SnapshotRetention    int             `json:"snapshotRetention,omitempty"`
	S3                   *ETCDSnapshotS3 `json:"s3,omitempty"`
}
