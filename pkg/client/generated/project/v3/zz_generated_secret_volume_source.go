package client

const (
	SecretVolumeSourceType             = "secretVolumeSource"
	SecretVolumeSourceFieldDefaultMode = "defaultMode"
	SecretVolumeSourceFieldDefaultUser = "defaultUser"
	SecretVolumeSourceFieldItems       = "items"
	SecretVolumeSourceFieldOptional    = "optional"
	SecretVolumeSourceFieldSecretName  = "secretName"
)

type SecretVolumeSource struct {
	DefaultMode *int64      `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	DefaultUser *int64      `json:"defaultUser,omitempty" yaml:"defaultUser,omitempty"`
	Items       []KeyToPath `json:"items,omitempty" yaml:"items,omitempty"`
	Optional    *bool       `json:"optional,omitempty" yaml:"optional,omitempty"`
	SecretName  string      `json:"secretName,omitempty" yaml:"secretName,omitempty"`
}
