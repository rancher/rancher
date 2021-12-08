package client

const (
	ComponentConditionType         = "componentCondition"
	ComponentConditionFieldError   = "error"
	ComponentConditionFieldMessage = "message"
	ComponentConditionFieldStatus  = "status"
	ComponentConditionFieldType    = "type"
)

type ComponentCondition struct {
	Error   string `json:"error,omitempty" yaml:"error,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	Status  string `json:"status,omitempty" yaml:"status,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
}
