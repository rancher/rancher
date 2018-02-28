package client

const (
	DownwardAPIProjectionType       = "downwardAPIProjection"
	DownwardAPIProjectionFieldItems = "items"
)

type DownwardAPIProjection struct {
	Items []DownwardAPIVolumeFile `json:"items,omitempty" yaml:"items,omitempty"`
}
