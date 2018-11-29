package client

const (
	ThanosSpecType           = "thanosSpec"
	ThanosSpecFieldBaseImage = "baseImage"
	ThanosSpecFieldGCS       = "gcs"
	ThanosSpecFieldPeers     = "peers"
	ThanosSpecFieldResources = "resources"
	ThanosSpecFieldS3        = "s3"
	ThanosSpecFieldSHA       = "sha"
	ThanosSpecFieldTag       = "tag"
	ThanosSpecFieldVersion   = "version"
)

type ThanosSpec struct {
	BaseImage string                `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	GCS       *ThanosGCSSpec        `json:"gcs,omitempty" yaml:"gcs,omitempty"`
	Peers     string                `json:"peers,omitempty" yaml:"peers,omitempty"`
	Resources *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	S3        *ThanosS3Spec         `json:"s3,omitempty" yaml:"s3,omitempty"`
	SHA       string                `json:"sha,omitempty" yaml:"sha,omitempty"`
	Tag       string                `json:"tag,omitempty" yaml:"tag,omitempty"`
	Version   string                `json:"version,omitempty" yaml:"version,omitempty"`
}
