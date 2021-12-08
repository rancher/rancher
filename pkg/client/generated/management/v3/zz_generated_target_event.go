package client

const (
	TargetEventType              = "targetEvent"
	TargetEventFieldEventType    = "eventType"
	TargetEventFieldResourceKind = "resourceKind"
)

type TargetEvent struct {
	EventType    string `json:"eventType,omitempty" yaml:"eventType,omitempty"`
	ResourceKind string `json:"resourceKind,omitempty" yaml:"resourceKind,omitempty"`
}
