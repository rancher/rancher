package client

const (
	EphemeralVolumeSourceType                     = "ephemeralVolumeSource"
	EphemeralVolumeSourceFieldVolumeClaimTemplate = "volumeClaimTemplate"
)

type EphemeralVolumeSource struct {
	VolumeClaimTemplate *PersistentVolumeClaimTemplate `json:"volumeClaimTemplate,omitempty" yaml:"volumeClaimTemplate,omitempty"`
}
