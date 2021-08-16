package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Name     string          `json:"name,omitempty"`
	NodeName string          `json:"nodeName,omitempty"`
	S3       *ETCDSnapshotS3 `json:"s3,omitempty"`
	// Changing the Generation is the only thing required to initiate a snapshot creation.
	Generation int `json:"generation,omitempty"`
}

type ETCDSnapshotRestore struct {
	ETCDSnapshot

	// Changing the Generation is the only thing required to initiate a snapshot creation.
	Generation int `json:"generation,omitempty"`
}

type ETCDSnapshot struct {
	Name      string          `json:"name,omitempty"`
	NodeName  string          `json:"nodeName,omitempty"`
	CreatedAt *metav1.Time    `json:"createdAt,omitempty"`
	Size      int64           `json:"size,omitempty"`
	S3        *ETCDSnapshotS3 `json:"s3,omitempty"`
}

type ETCD struct {
	DisableSnapshots     bool            `json:"disableSnapshots,omitempty"`
	SnapshotScheduleCron string          `json:"snapshotScheduleCron,omitempty"`
	SnapshotRetention    int             `json:"snapshotRetention,omitempty"`
	S3                   *ETCDSnapshotS3 `json:"s3,omitempty"`
}
