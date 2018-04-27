package v3

import (
	"github.com/rancher/norman/condition"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Catalog struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec   CatalogSpec   `json:"spec"`
	Status CatalogStatus `json:"status"`
}

type CatalogSpec struct {
	Description string `json:"description"`
	URL         string `json:"url,omitempty" norman:"required"`
	Branch      string `json:"branch,omitempty"`
	CatalogKind string `json:"catalogKind,omitempty"`
}

type CatalogStatus struct {
	LastRefreshTimestamp string `json:"lastRefreshTimestamp,omitempty"`
	Commit               string `json:"commit,omitempty"`
	// helmVersionCommits records hash of each helm template version
	HelmVersionCommits map[string]VersionCommits `json:"helmVersionCommits,omitempty"`
	Conditions         []CatalogCondition        `json:"conditions,omitempty"`
}

var (
	CatalogConditionRefreshed condition.Cond = "Refreshed"
)

type CatalogCondition struct {
	// Type of cluster condition.
	Type ClusterConditionType `json:"type"`
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

type VersionCommits struct {
	Value map[string]string `json:"Value,omitempty"`
}

type Template struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec   TemplateSpec   `json:"spec"`
	Status TemplateStatus `json:"status"`
}

type TemplateSpec struct {
	DisplayName              string `json:"displayName"`
	CatalogID                string `json:"catalogId,omitempty" norman:"type=reference[catalog]"`
	DefaultTemplateVersionID string `json:"defaultTemplateVersionId,omitempty" norman:"type=reference[templateVersion]"`

	Description    string `json:"description,omitempty"`
	DefaultVersion string `json:"defaultVersion,omitempty" yaml:"default_version,omitempty"`
	Path           string `json:"path,omitempty"`
	Maintainer     string `json:"maintainer,omitempty"`
	ProjectURL     string `json:"projectURL,omitempty" yaml:"project_url,omitempty"`
	UpgradeFrom    string `json:"upgradeFrom,omitempty"`
	FolderName     string `json:"folderName,omitempty"`
	Icon           string `json:"icon,omitempty"`
	IconFilename   string `json:"iconFilename,omitempty"`
	Readme         string `json:"readme,omitempty"`

	Categories []string              `json:"categories,omitempty"`
	Versions   []TemplateVersionSpec `json:"versions,omitempty"`

	Category string `json:"category,omitempty"`
}

type TemplateStatus struct {
}

type TemplateVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec   TemplateVersionSpec   `json:"spec"`
	Status TemplateVersionStatus `json:"status"`
}

type TemplateVersionSpec struct {
	ExternalID          string            `json:"externalId,omitempty"`
	Version             string            `json:"version,omitempty"`
	RancherVersion      string            `json:"rancherVersion,omitempty"`
	KubeVersion         string            `json:"kubeVersion,omitempty"`
	Readme              string            `json:"readme,omitempty"`
	AppReadme           string            `json:"appReadme,omitempty"`
	UpgradeVersionLinks map[string]string `json:"upgradeVersionLinks,omitempty"`
	Digest              string            `json:"digest,omitempty"`

	Files     map[string]string `json:"files,omitempty"`
	Questions []Question        `json:"questions,omitempty"`
}

type TemplateVersionStatus struct {
}

type File struct {
	Name     string `json:"name,omitempty"`
	Contents string `json:"contents,omitempty"`
}

type Question struct {
	Variable          string        `json:"variable,omitempty" yaml:"variable,omitempty"`
	Label             string        `json:"label,omitempty" yaml:"label,omitempty"`
	Description       string        `json:"description,omitempty" yaml:"description,omitempty"`
	Type              string        `json:"type,omitempty" yaml:"type,omitempty"`
	Required          bool          `json:"required,omitempty" yaml:"required,omitempty"`
	Default           string        `json:"default,omitempty" yaml:"default,omitempty"`
	Group             string        `json:"group,omitempty" yaml:"group,omitempty"`
	MinLength         int           `json:"minLength,omitempty" yaml:"min_length,omitempty"`
	MaxLength         int           `json:"maxLength,omitempty" yaml:"max_length,omitempty"`
	Min               int           `json:"min,omitempty" yaml:"min,omitempty"`
	Max               int           `json:"max,omitempty" yaml:"max,omitempty"`
	Options           []string      `json:"options,omitempty" yaml:"options,omitempty"`
	ValidChars        string        `json:"validChars,omitempty" yaml:"valid_chars,omitempty"`
	InvalidChars      string        `json:"invalidChars,omitempty" yaml:"invalid_chars,omitempty"`
	Subquestions      []SubQuestion `json:"subquestions,omitempty" yaml:"subquestions,omitempty"`
	ShowIf            string        `json:"showIf,omitempty" yaml:"show_if,omitempty"`
	ShowSubquestionIf string        `json:"showSubquestionIf,omitempty" yaml:"show_subquestion_if,omitempty"`
}

type SubQuestion struct {
	Variable     string   `json:"variable,omitempty" yaml:"variable,omitempty"`
	Label        string   `json:"label,omitempty" yaml:"label,omitempty"`
	Description  string   `json:"description,omitempty" yaml:"description,omitempty"`
	Type         string   `json:"type,omitempty" yaml:"type,omitempty"`
	Required     bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Default      string   `json:"default,omitempty" yaml:"default,omitempty"`
	Group        string   `json:"group,omitempty" yaml:"group,omitempty"`
	MinLength    int      `json:"minLength,omitempty" yaml:"min_length,omitempty"`
	MaxLength    int      `json:"maxLength,omitempty" yaml:"max_length,omitempty"`
	Min          int      `json:"min,omitempty" yaml:"min,omitempty"`
	Max          int      `json:"max,omitempty" yaml:"max,omitempty"`
	Options      []string `json:"options,omitempty" yaml:"options,omitempty"`
	ValidChars   string   `json:"validChars,omitempty" yaml:"valid_chars,omitempty"`
	InvalidChars string   `json:"invalidChars,omitempty" yaml:"invalid_chars,omitempty"`
	ShowIf       string   `json:"showIf,omitempty" yaml:"show_if,omitempty"`
}

type TemplateContent struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Data string `json:"data,omitempty"`
}
