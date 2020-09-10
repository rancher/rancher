package client

const (
	EphemeralVolumeSourceType                     = "ephemeralVolumeSource"
	EphemeralVolumeSourceFieldReadOnly            = "readOnly"
	EphemeralVolumeSourceFieldVolumeClaimTemplate = "volumeClaimTemplate"
)

type EphemeralVolumeSource struct {
	ReadOnly            bool                           `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeClaimTemplate *PersistentVolumeClaimTemplate `json:"volumeClaimTemplate,omitempty" yaml:"volumeClaimTemplate,omitempty"`
}
