package client

const (
	StatusType            = "status"
	StatusFieldAPIVersion = "apiVersion"
	StatusFieldCode       = "code"
	StatusFieldDetails    = "details"
	StatusFieldKind       = "kind"
	StatusFieldListMeta   = "metadata"
	StatusFieldMessage    = "message"
	StatusFieldReason     = "reason"
	StatusFieldStatus     = "status"
)

type Status struct {
	APIVersion string         `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Code       *int64         `json:"code,omitempty" yaml:"code,omitempty"`
	Details    *StatusDetails `json:"details,omitempty" yaml:"details,omitempty"`
	Kind       string         `json:"kind,omitempty" yaml:"kind,omitempty"`
	ListMeta   *ListMeta      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Message    string         `json:"message,omitempty" yaml:"message,omitempty"`
	Reason     string         `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status     string         `json:"status,omitempty" yaml:"status,omitempty"`
}
