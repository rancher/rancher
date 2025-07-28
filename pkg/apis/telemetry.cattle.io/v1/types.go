package v1

import (
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceConditionDone        condition.Cond = "Done"
	ResourceConditionFailure     condition.Cond = "Failure"
	ResourceConditionProgressing condition.Cond = "Progressing"
	ResourceConditionReady       condition.Cond = "Ready"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.secretType`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SecretRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretRequestSpec   `json:"spec,omitempty"`
	Status SecretRequestStatus `json:"status,omitempty"`
}

// SecretRequestSpec defines the secret type being requested, and the target where the secret will be created
type SecretRequestSpec struct {
	// TODO: probably an enum matching pre-defined telemetry export targets
	SecretType      string                  `json:"secretType"` // This is directly tied to instances of secrets that are registered.
	TargetSecretRef *corev1.SecretReference `json:"targetSecretRef"`
}

type SecretRequestStatus struct {
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// +optional
	LastSyncTS *metav1.Time `json:"lastSyncTS"`
}

func (sr *SecretRequest) HasCondition(matchCond condition.Cond) bool {
	conditions := sr.Status.Conditions
	for _, cond := range conditions {
		if cond.Type == string(matchCond) {
			return true
		}
	}

	return false
}
