package client

const (
	ResourceHealthType            = "resourceHealth"
	ResourceHealthFieldHealth     = "health"
	ResourceHealthFieldResourceID = "resourceID"
)

type ResourceHealth struct {
	Health     string `json:"health,omitempty" yaml:"health,omitempty"`
	ResourceID string `json:"resourceID,omitempty" yaml:"resourceID,omitempty"`
}
