package v3

import (
	v1 "k8s.io/api/core/v1"
)

// Note: This struct is largely a mirror of the upstream 'metav1.Condition' type, with one key difference: The LastUpdateTime field.
// The 'condition' packages within both norman and wrangler attempt to set this field when updating the condition,
// and will panic if they are unable to do so (due to their use of reflection). Due to this, as well as the absence of other
// utility packages for working with conditions in Rancher, simply swapping out this Condition struct with the
// upstream 'metav1.Condition' struct is not a trivial change and likely needs to be evaluated on a per CRD basis.

type Condition struct {
	// type of condition in CamelCase or in foo.example.com/CamelCase.
	// ---
	// Many .condition.type values are consistent across resources like
	// Available, but because arbitrary conditions can be useful
	// (see .node.status.conditions), the ability to deconflict is important.
	// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	// +kubebuilder:validation:MaxLength=316
	Type string `json:"type"`
	// status of the condition, one of True, False, Unknown.
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status v1.ConditionStatus `json:"status"`
	// lastUpdateTime of this condition. This is incremented if the resource
	// is updated for any reason. This could be when the underlying condition
	// changed, but may also be updated if other fields are modified
	// (Message, Reason, etc.).
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// lastTransitionTime is the last time the condition transitioned from
	// one status to another. This should be when the underlying condition
	// changed. If that is not known, then using the time when the API field
	// changed is acceptable.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// reason contains a programmatic identifier indicating the reason for
	// the condition's last transition. Producers of specific condition types
	// may define expected values and meanings for this field, and whether
	// the values are considered a guaranteed API. The value should be
	// a CamelCase string.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern=`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`
	Reason string `json:"reason,omitempty"`
	// message is a human readable message indicating details
	// about the transition.
	// This may be an empty string.
	// +optional
	// +kubebuilder:validation:MaxLength=32768
	Message string `json:"message,omitempty"`
}
