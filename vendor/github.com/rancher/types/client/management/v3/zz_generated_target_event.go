package client

const (
	TargetEventType              = "targetEvent"
	TargetEventFieldResourceKind = "resourceKind"
	TargetEventFieldType         = "type"
)

type TargetEvent struct {
	ResourceKind string `json:"resourceKind,omitempty"`
	Type         string `json:"type,omitempty"`
}
