package v3

import (
	"github.com/rancher/norman/condition"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComposeConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec   ComposeSpec   `json:"spec,omitempty"`
	Status ComposeStatus `json:"status,omitempty"`
}

type ComposeSpec struct {
	RancherCompose string `json:"rancherCompose,omitempty"`
}

type ComposeStatus struct {
	Conditions []ComposeCondition `json:"conditions,omitempty"`
}

var (
	ComposeConditionExecuted condition.Cond = "Executed"
)

type ComposeCondition struct {
	// Type of cluster condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}
