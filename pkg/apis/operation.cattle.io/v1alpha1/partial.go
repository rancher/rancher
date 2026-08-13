package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Operation is a kind-agnostic view of any operation.cattle.io resource. Every operation kind inlines
// OperationSpec and OperationStatus, so any operation can be decoded into an Operation in order to
// reason about the shared fields without knowing its concrete kind.
//
// Operation is not itself a stored resource; it exists so that generic consumers - admission control,
// preemption - can be written once rather than per kind. ObjectMeta is deliberately a named field
// rather than embedded: embedding it would cause the wrangler client generator to treat Operation as
// a real resource and emit a client for it.
type Operation struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object's metadata.
	// +optional
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the fields shared by every operation's spec.
	// +optional
	Spec OperationSpec `json:"spec,omitempty"`

	// Status holds the fields shared by every operation's status.
	// +optional
	Status OperationStatus `json:"status,omitempty"`
}

// Key returns the operation's identity in the form used by RestoreRequiredAnnotation and
// CanceledByAnnotation.
func (o *Operation) Key() string {
	return ObjectRefKey(o.Kind, o.Metadata.Namespace, o.Metadata.Name)
}

// SameAs reports whether two operations are the same object.
func (o *Operation) SameAs(other *Operation) bool {
	if o == nil || other == nil {
		return o == other
	}

	return o.Kind == other.Kind &&
		o.Metadata.Namespace == other.Metadata.Namespace &&
		o.Metadata.Name == other.Metadata.Name
}

// SameCluster reports whether two cluster references point at the same cluster. A reference with no
// namespace matches one in any namespace: the namespace is optional for cluster-scoped clusters, and
// treating a missing namespace as a match keeps operations serialized rather than letting an incomplete
// reference slip past.
//
// Admission control and the operation controllers must agree on this, or an operation the webhook
// admitted would not be preempted, so both use this function.
func SameCluster(left, right *corev1.ObjectReference) bool {
	if left == nil || right == nil {
		return left == right
	}

	leftGroup, err := schema.ParseGroupVersion(left.APIVersion)
	if err != nil {
		return false
	}

	rightGroup, err := schema.ParseGroupVersion(right.APIVersion)
	if err != nil {
		return false
	}

	if leftGroup.Group != rightGroup.Group || left.Kind != right.Kind || left.Name != right.Name {
		return false
	}

	return left.Namespace == "" || right.Namespace == "" || left.Namespace == right.Namespace
}

// ToOperation converts a concrete operation into the kind-agnostic view. Objects served from an
// informer have an empty TypeMeta, so when the kind is absent it is recovered from the Go type, whose
// name is the kind for every generated operation type.
func ToOperation(obj any) (*Operation, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("unable to encode %T: %w", obj, err)
	}

	operation := &Operation{}
	if err := json.Unmarshal(data, operation); err != nil {
		return nil, fmt.Errorf("unable to decode %T as an operation: %w", obj, err)
	}

	if operation.Kind == "" {
		operation.Kind = kindOf(obj)
	}

	return operation, nil
}

// kindOf returns the name of the given value's type, dereferencing pointers.
func kindOf(obj any) string {
	objType := reflect.TypeOf(obj)
	for objType != nil && objType.Kind() == reflect.Pointer {
		objType = objType.Elem()
	}

	if objType == nil {
		return ""
	}

	return objType.Name()
}
