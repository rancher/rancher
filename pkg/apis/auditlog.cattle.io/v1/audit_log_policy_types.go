package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuditLogPolicyStatusCondition string
type FilterAction string

const (
	AuditLogPolicyStatusConditionUnknown  AuditLogPolicyStatusCondition = ""
	AuditLogPolicyStatusConditionDisabled AuditLogPolicyStatusCondition = "Disabled"
	AuditLogPolicyStatusConditionEnabled  AuditLogPolicyStatusCondition = "Enabled"
	AuditLogPolicyStatusConditionInvalid  AuditLogPolicyStatusCondition = "Invalid"

	FilterActionUnknown FilterAction = ""
	FilterActionAllow   FilterAction = "allow"
	FilterActionDeny    FilterAction = "deny"
)

type Filter struct {
	Action     FilterAction `json:"action"`
	RequestURI string       `json:"requestURI"`
}

type Redaction struct {
	Headers []string `json:"headers"`
	Paths   []string `json:"paths"`
	Keys    []string `json:"keys"`
}

type Verbosity struct {
	Headers bool `json:"headers"`
	Body    bool `json:"body"`
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

	Request  Verbosity `json:"request"`
	Response Verbosity `json:"response"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuditLogPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuditLogPolicySpec   `json:"spec"`
	Status AuditLogPolicyStatus `json:"status,omitempty"`
}

type AuditLogPolicySpec struct {
	Enabled              bool         `json:"enabled"`
	Filters              []Filter     `json:"filters"`
	AdditionalRedactions []Redaction  `json:"additionalRedactions"`
	Verbosity            LogVerbosity `json:"verbosity"`
}

type AuditLogPolicyStatus struct {
	Condition AuditLogPolicyStatusCondition `json:"condition,omitempty"`
	Message   string                        `json:"message,omitempty"`
}
