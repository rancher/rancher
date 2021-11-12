package client

const (
	EventRuleType              = "eventRule"
	EventRuleFieldEventType    = "eventType"
	EventRuleFieldResourceKind = "resourceKind"
)

type EventRule struct {
	EventType    string `json:"eventType,omitempty" yaml:"eventType,omitempty"`
	ResourceKind string `json:"resourceKind,omitempty" yaml:"resourceKind,omitempty"`
}
