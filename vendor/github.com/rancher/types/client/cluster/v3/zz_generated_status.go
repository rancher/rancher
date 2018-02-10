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
	APIVersion string         `json:"apiVersion,omitempty"`
	Code       *int64         `json:"code,omitempty"`
	Details    *StatusDetails `json:"details,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	ListMeta   *ListMeta      `json:"metadata,omitempty"`
	Message    string         `json:"message,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	Status     string         `json:"status,omitempty"`
}
