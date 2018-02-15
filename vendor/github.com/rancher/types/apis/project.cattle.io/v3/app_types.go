package v3

import (
	"github.com/rancher/norman/types"
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
	Prune            bool              `json:"prune,omitempty"`
	Tag              map[string]string `json:"tag,omitempty"`
	User             string            `json:"user,omitempty"`
	Groups           []string          `json:"groups,omitempty"`
}

type AppStatus struct {
	Releases []ReleaseInfo `json:"releases,omitempty"`
}

type ReleaseInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	CreateTimestamp   string `json:"createTimestamp"`
	ModifiedAt        string `json:"modifiedAt"`
	TemplateVersionID string `json:"templateVersionId"`
}
