package client

const (
	SecretVolumeSourceType             = "secretVolumeSource"
	SecretVolumeSourceFieldDefaultMode = "defaultMode"
	SecretVolumeSourceFieldItems       = "items"
	SecretVolumeSourceFieldOptional    = "optional"
	SecretVolumeSourceFieldSecretId    = "secretId"
)

type SecretVolumeSource struct {
	DefaultMode *int64      `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	Items       []KeyToPath `json:"items,omitempty" yaml:"items,omitempty"`
	Optional    *bool       `json:"optional,omitempty" yaml:"optional,omitempty"`
	SecretId    string      `json:"secretId,omitempty" yaml:"secretId,omitempty"`
}
