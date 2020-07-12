package client

const (
	AllowedCSIDriverType      = "allowedCSIDriver"
	AllowedCSIDriverFieldName = "name"
)

type AllowedCSIDriver struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
