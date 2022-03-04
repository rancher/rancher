package client

const (
	PodOSType      = "podOS"
	PodOSFieldName = "name"
)

type PodOS struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
