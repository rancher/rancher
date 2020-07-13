package client

const (
	ThanosSpecType                     = "thanosSpec"
	ThanosSpecFieldBaseImage           = "baseImage"
	ThanosSpecFieldImage               = "image"
	ThanosSpecFieldListenLocal         = "listenLocal"
	ThanosSpecFieldObjectStorageConfig = "objectStorageConfig"
	ThanosSpecFieldResources           = "resources"
	ThanosSpecFieldSHA                 = "sha"
	ThanosSpecFieldTag                 = "tag"
	ThanosSpecFieldTracingConfig       = "tracingConfig"
	ThanosSpecFieldVersion             = "version"
)

type ThanosSpec struct {
	BaseImage           string                `json:"baseImage,omitempty" yaml:"baseImage,omitempty"`
	Image               string                `json:"image,omitempty" yaml:"image,omitempty"`
	ListenLocal         bool                  `json:"listenLocal,omitempty" yaml:"listenLocal,omitempty"`
	ObjectStorageConfig *SecretKeySelector    `json:"objectStorageConfig,omitempty" yaml:"objectStorageConfig,omitempty"`
	Resources           *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	SHA                 string                `json:"sha,omitempty" yaml:"sha,omitempty"`
	Tag                 string                `json:"tag,omitempty" yaml:"tag,omitempty"`
	TracingConfig       *SecretKeySelector    `json:"tracingConfig,omitempty" yaml:"tracingConfig,omitempty"`
	Version             string                `json:"version,omitempty" yaml:"version,omitempty"`
}
