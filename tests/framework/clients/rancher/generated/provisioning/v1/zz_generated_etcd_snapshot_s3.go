package client

const (
	ETCDSnapshotS3Type                     = "etcdSnapshotS3"
	ETCDSnapshotS3FieldBucket              = "bucket"
	ETCDSnapshotS3FieldCloudCredentialName = "cloudCredentialName"
	ETCDSnapshotS3FieldEndpoint            = "endpoint"
	ETCDSnapshotS3FieldEndpointCA          = "endpointCA"
	ETCDSnapshotS3FieldFolder              = "folder"
	ETCDSnapshotS3FieldRegion              = "region"
	ETCDSnapshotS3FieldSkipSSLVerify       = "skipSSLVerify"
)

type ETCDSnapshotS3 struct {
	Bucket              string `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	CloudCredentialName string `json:"cloudCredentialName,omitempty" yaml:"cloudCredentialName,omitempty"`
	Endpoint            string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	EndpointCA          string `json:"endpointCA,omitempty" yaml:"endpointCA,omitempty"`
	Folder              string `json:"folder,omitempty" yaml:"folder,omitempty"`
	Region              string `json:"region,omitempty" yaml:"region,omitempty"`
	SkipSSLVerify       bool   `json:"skipSSLVerify,omitempty" yaml:"skipSSLVerify,omitempty"`
}
