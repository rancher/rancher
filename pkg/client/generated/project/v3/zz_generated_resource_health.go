package client

const (
	ResourceHealthType            = "resourceHealth"
	ResourceHealthFieldHealth     = "health"
	ResourceHealthFieldMessage    = "message"
	ResourceHealthFieldResourceID = "resourceID"
)

type ResourceHealth struct {
	Health     string `json:"health,omitempty" yaml:"health,omitempty"`
	Message    string `json:"message,omitempty" yaml:"message,omitempty"`
	ResourceID string `json:"resourceID,omitempty" yaml:"resourceID,omitempty"`
}
