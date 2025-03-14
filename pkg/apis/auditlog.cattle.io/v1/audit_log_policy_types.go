package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuditLogPolicyStatusCondition string
type FilterAction string

const (
	AuditLogPolicyStatusConditionUnknown  AuditLogPolicyStatusCondition = ""
	AuditLogPolicyStatusConditionActive   AuditLogPolicyStatusCondition = "active"
	AuditLogPolicyStatusConditionInactive AuditLogPolicyStatusCondition = "inactive"
	AuditLogPolicyStatusConditionInvalid  AuditLogPolicyStatusCondition = "invalid"

	FilterActionUnknown FilterAction = ""
	FilterActionAllow   FilterAction = "allow"
	FilterActionDeny    FilterAction = "deny"
)

type Filter struct {
	Action     FilterAction `json:"action,omitempty"`
	RequestURI string       `json:"requestURI,omitempty"`
}

type Redaction struct {
	Headers []string `json:"headers,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

type Verbosity struct {
	Headers bool `json:"headers,omitempty"`
	Body    bool `json:"body,omitempty"`
}

type Level int

const (
	LevelNull Level = iota
	LevelMetadata
	LevelRequest
	LevelRequestResponse
)

type LogVerbosity struct {
	Level Level `json:"level"`

	Request  Verbosity `json:"request,omitempty"`
	Response Verbosity `json:"response,omitempty"`
}

// +genclient
// +kubebuilder:printcolumn:name="Enabled",type=string,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Condition",type=string,JSONPath=`.status.condition`
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuditLogPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuditLogPolicySpec   `json:"spec"`
	Status AuditLogPolicyStatus `json:"status,omitempty"`
}

type AuditLogPolicySpec struct {
	Enabled bool `json:"enabled"`

	// Filters described what are explicitly allowed and denied. Leave empty if all logs should be allowed.
	Filters              []Filter     `json:"filters,omitempty"`
	AdditionalRedactions []Redaction  `json:"additionalRedactions,omitempty"`
	Verbosity            LogVerbosity `json:"verbosity,omitempty"`
}

type AuditLogPolicyStatus struct {
	Condition AuditLogPolicyStatusCondition `json:"condition,omitempty"`
	Message   string                        `json:"message,omitempty"`
}
