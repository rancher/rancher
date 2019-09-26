package client

const (
	ThanosSpecType                     = "thanosSpec"
	ThanosSpecFieldBaseImage           = "baseImage"
	ThanosSpecFieldImage               = "image"
	ThanosSpecFieldObjectStorageConfig = "objectStorageConfig"
	ThanosSpecFieldResources           = "resources"
	ThanosSpecFieldSHA                 = "sha"
	ThanosSpecFieldTag                 = "tag"
	ThanosSpecFieldVersion             = "version"
)

type ThanosSpec struct {
	BaseImage           string                `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	Image               string                `json:"image,omitempty" yaml:"image,omitempty"`
	ObjectStorageConfig *SecretKeySelector    `json:"objectStorageConfig,omitempty" yaml:"objectStorageConfig,omitempty"`
	Resources           *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	SHA                 string                `json:"sha,omitempty" yaml:"sha,omitempty"`
	Tag                 string                `json:"tag,omitempty" yaml:"tag,omitempty"`
	Version             string                `json:"version,omitempty" yaml:"version,omitempty"`
}
