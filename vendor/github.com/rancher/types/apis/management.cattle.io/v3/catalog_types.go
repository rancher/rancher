package v3

import (
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
	URL         string `json:"url,omitempty"`
	Branch      string `json:"branch,omitempty"`
	CatalogKind string `json:"catalogKind,omitempty"`
}

type CatalogStatus struct {
	LastRefreshTimestamp string `json:"lastRefreshTimestamp,omitempty"`
	Commit               string `json:"commit,omitempty"`
	// helmVersionCommits records hash of each helm template version
	HelmVersionCommits map[string]VersionCommits `json:"helmVersionCommits,omitempty"`
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
	CatalogID                string `json:"catalogId,omitempty" norman:"type=reference[catalog]"`
	DefaultTemplateVersionID string `json:"defaultTemplateVersionId,omitempty" norman:"type=reference[templateVersion]"`

	IsSystem       string `json:"isSystem,omitempty"`
	Description    string `json:"description,omitempty"`
	DefaultVersion string `json:"defaultVersion,omitempty" yaml:"default_version,omitempty"`
	Path           string `json:"path,omitempty"`
	Maintainer     string `json:"maintainer,omitempty"`
	License        string `json:"license,omitempty"`
	ProjectURL     string `json:"projectURL,omitempty" yaml:"project_url,omitempty"`
	UpgradeFrom    string `json:"upgradeFrom,omitempty"`
	FolderName     string `json:"folderName,omitempty"`
	Base           string `json:"templateBase"`
	Icon           string `json:"icon,omitempty"`
	IconFilename   string `json:"iconFilename,omitempty"`
	Readme         string `json:"readme,omitempty"`

	Categories []string              `json:"categories,omitempty"`
	Versions   []TemplateVersionSpec `json:"versions,omitempty"`

	Category string `json:"category,omitempty"`
}

type TemplateStatus struct {
	// todo:
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
	ExternalID            string            `json:"externalId,omitempty"`
	Revision              *int              `json:"revision,omitempty"`
	Version               string            `json:"version,omitempty"`
	MinimumRancherVersion string            `json:"minimumRancherVersion,omitempty" yaml:"minimum_rancher_version,omitempty"`
	MaximumRancherVersion string            `json:"maximumRancherVersion,omitempty" yaml:"maximum_rancher_version,omitempty"`
	UpgradeFrom           string            `json:"upgradeFrom,omitempty" yaml:"upgrade_from,omitempty"`
	Readme                string            `json:"readme,omitempty"`
	UpgradeVersionLinks   map[string]string `json:"upgradeVersionLinks,omitempty"`

	Files     []File     `json:"files,omitempty"`
	Questions []Question `json:"questions,omitempty"`
}

type TemplateVersionStatus struct {
	// todo
}

type File struct {
	Name     string `json:"name,omitempty"`
	Contents string `json:"contents,omitempty"`
}

type Question struct {
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
}
