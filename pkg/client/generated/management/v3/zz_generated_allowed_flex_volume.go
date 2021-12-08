package client

const (
	AllowedFlexVolumeType        = "allowedFlexVolume"
	AllowedFlexVolumeFieldDriver = "driver"
)

type AllowedFlexVolume struct {
	Driver string `json:"driver,omitempty" yaml:"driver,omitempty"`
}
