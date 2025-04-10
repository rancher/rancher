package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuditPolicyStatusCondition string
type FilterAction string

const (
	AuditPolicyStatusConditionUnknown  AuditPolicyStatusCondition = "Unknown"
	AuditPolicyStatusConditionActive   AuditPolicyStatusCondition = "Active"
	AuditPolicyStatusConditionInactive AuditPolicyStatusCondition = "Inactive"
	AuditPolicyStatusConditionInvalid  AuditPolicyStatusCondition = "Invalid"

	FilterActionUnknown FilterAction = ""
	FilterActionAllow   FilterAction = "allow"
	FilterActionDeny    FilterAction = "deny"
)

// Filter provides values used to filter out audit logs.
type Filter struct {
	// Action defines what happens
	Action FilterAction `json:"action,omitempty"`

	// RequestURI is a regular expression used to match against the url of the log request.
	RequestURI string `json:"requestURI,omitempty"`
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
	// LevelNull indicates that no Request or Resopnse data beyond what is included in the audit log metadata. A
	// LogVerbosity with LevelNull is the same as the zero value for LogVerbosity or LogVerbosity{}.
	LevelNull Level = iota

	// LevelHeaders indicates that along with the default audit log metadata, the request and response headers will
	// also be included. A LogVerbosity with LevelHeaders is the same as the following LogVerbosity:
	//
	// LogVerbosity {
	//     Request: {
	//         Headers: true
	//     },
	//     Response: {
	//         Headers: true
	//     },
	// }
	LevelHeaders

	// LevelRequest indicates that along with the default audit log metadata and headers, the request body will also be
	// included. A LogVerbosity with LevelHeaders is the same as the following LogVerbosity:
	//
	// LogVerbosity {
	//     Request: {
	//         Headers: true
	//         Body: true,
	//     },
	//     Response: {
	//         Headers: true
	//     },
	// }
	LevelRequest

	// LevelRequestResponse indicates that along with the default audit log metadata and headers, the request and
	// response bodies will also be included. A LogVerbosity with LevelHeaders is the same as the following
	// LogVerbosity:
	//
	// LogVerbosity {
	//     Request: {
	//         Headers: true
	//         Body: true,
	//     },
	//     Response: {
	//         Headers: true
	//         Body: true,
	//     },
	// }
	LevelRequestResponse
)

// LogVerbosity defines what is included in an audit log. Log metadata (includeing RequestURI, user info, etc) is always present.
type LogVerbosity struct {
	// Level is carried over from the previous implementation of audit logging, and provides a shorthand for defining
	// LogVerbosities. When Level is not LevelNull, Request and Reponse are ignored.
	Level Level `json:"level"`

	Request  Verbosity `json:"request,omitempty"`
	Response Verbosity `json:"response,omitempty"`
}

// +genclient
// +kubebuilder:printcolumn:name="Enabled",type=string,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Condition",type=string,JSONPath=`.status.condition`
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuditPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuditPolicySpec   `json:"spec"`
	Status AuditPolicyStatus `json:"status,omitempty"`
}

type AuditPolicySpec struct {
	Enabled bool `json:"enabled"`

	// Filters described what are explicitly allowed and denied. Leave empty if all logs should be allowed.
	Filters []Filter `json:"filters,omitempty"`

	// AdditionalRedactions details additional informatino to be redacted. These redactions are only applied to logs
	// that are allowed by the defiend Filters. Note that if no filters are defined, these redactions will apply to all
	// logs.
	AdditionalRedactions []Redaction `json:"additionalRedactions,omitempty"`

	Verbosity LogVerbosity `json:"verbosity,omitempty"`
}

type AuditPolicyStatus struct {
	Condition AuditPolicyStatusCondition `json:"condition,omitempty"`
	Message   string                     `json:"message,omitempty"`
}
