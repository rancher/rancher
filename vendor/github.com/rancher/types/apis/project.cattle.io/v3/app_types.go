package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type App struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

type AppSpec struct {
	ProjectName      string            `json:"projectName,omitempty" norman:"type=reference[/v3/schemas/project]"`
	Description      string            `json:"description,omitempty"`
	InstallNamespace string            `json:"installNamespace,omitempty"`
	ExternalID       string            `json:"externalId,omitempty"`
	Templates        map[string]string `json:"templates,omitempty"`
	Answers          map[string]string `json:"answers,omitempty"`
	AnswerValues     string            `json:"answerValues,omitempty"`
}

var (
	AppConditionInstalled condition.Cond = "installed"
)

type AppStatus struct {
	Releases   []ReleaseInfo  `json:"releases,omitempty"`
	Conditions []AppCondition `json:"conditions,omitempty"`
}

type AppCondition struct {
	// Type of cluster condition.
	Type condition.Cond `json:"type"`
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

type ReleaseInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	CreateTimestamp   string `json:"createTimestamp"`
	ModifiedAt        string `json:"modifiedAt"`
	TemplateVersionID string `json:"templateVersionId"`
}
