package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuditPolicyConditionType string
type FilterAction string

const (
	AuditPolicyConditionTypeUnknown AuditPolicyConditionType = "Unknown"
	AuditPolicyConditionTypeActive  AuditPolicyConditionType = "Active"
	AuditPolicyConditionTypeValid   AuditPolicyConditionType = "Valid"

	FilterActionUnknown FilterAction = ""
	FilterActionAllow   FilterAction = "allow"
	FilterActionDeny    FilterAction = "deny"
)

// Filter provides values used to filter out audit logs.
type Filter struct {
	// Action defines what happens
	Action FilterAction `json:"action,omitempty"`

	// RequestURI is a regular expression used to match against the url of the log request. For exapmle, the Filter:
	//
	// Filter {
	//     Action: Allow.
	//     REquestURI: "/foo/.*"
	// }
	//
	// would allow logs sent to "/foo/some/endpoint" but not "/foo" or "/foobar".
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
// +genclient:nonNamespaced
// +kubebuilder:printcolumn:name="Enabled",type=string,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Active",type=string,JSONPath=`.status.conditions[?(@.type == "Active")].status`
// +kubebuilder:printcolumn:name="Valid",type=string,JSONPath=`.status.conditions[?(@.type == "Valid")].status`
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

type AuditPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuditPolicySpec   `json:"spec"`
	Status AuditPolicyStatus `json:"status,omitempty"`
}

type AuditPolicySpec struct {
	Enabled bool `json:"enabled"`

	// Filters described what logs are explicitly allowed and denied. Leave empty if all logs should be allowed. The
	// Allow action has higher precedence than Deny. So if there are multiple filters that match a log and at least one
	// Allow, the log will be allowed.
	Filters []Filter `json:"filters,omitempty"`

	// AdditionalRedactions details additional informatino to be redacted. If there are any Filers defined in the same
	// policy, these Redactions will only be applied to logs that are Allowed by those filters. If there are no
	// Filters, the redactions will be applied to all logs.
	AdditionalRedactions []Redaction `json:"additionalRedactions,omitempty"`

	// Verbosity defines how much data to collect from each log. The end verbosity for a log is calculated as a merge
	// of each policy that Allows a log (including plicies with no Filters). For example, take the two policie specs
	// below:
	//
	// AuditPolicySpec {
	//     Enabled: True,
	//     Verbosity: LogVerbosity {
	//         Request: Verbosity {
	//             Body: True,
	//         },
	//     },
	// }
	//
	// AuditPolicySpec {
	//     Enabled: True,
	//     Filters: []Filters{
	//         {
	//             Action: "allow",
	//             RequestURI: "/foo"
	//         },
	//     },
	//     Verbosity: LogVerbosity {
	//         Response: Verbosity {
	//             Body: True,
	//         },
	//     },
	// }
	//
	// A request to the "/foo" endpoint will log both the request and response bodies, but a request to "/bar" will
	// only log the request body.
	Verbosity LogVerbosity `json:"verbosity,omitempty"`
}

type AuditPolicyStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
