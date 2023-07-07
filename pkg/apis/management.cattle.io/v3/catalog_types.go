package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
	Description    string         `json:"description"`
	URL            string         `json:"url,omitempty" norman:"required"`
	Branch         string         `json:"branch,omitempty"`
	CatalogKind    string         `json:"catalogKind,omitempty"`
	Username       string         `json:"username,omitempty"`
	Password       string         `json:"password,omitempty" norman:"type=password"`
	HelmVersion    string         `json:"helmVersion,omitempty" norman:"noupdate"`
	CatalogSecrets CatalogSecrets `json:"catalogSecrets" norman:"nocreate,noupdate"`
}

type CatalogStatus struct {
	LastRefreshTimestamp string             `json:"lastRefreshTimestamp,omitempty"`
	Commit               string             `json:"commit,omitempty"`
	Conditions           []CatalogCondition `json:"conditions,omitempty"`

	// Deprecated: should no longer be in use. If a Catalog CR is encountered with this field
	// populated, it will be set to nil.
	HelmVersionCommits map[string]VersionCommits `json:"helmVersionCommits,omitempty"`
	CredentialSecret   string                    `json:"credentialSecret,omitempty" norman:"nocreate,noupdate"` // Deprecated: use CatalogSpec.CatalogSecrets.CredentialSecret instead
}

var (
	CatalogConditionRefreshed       condition.Cond = "Refreshed"
	CatalogConditionUpgraded        condition.Cond = "Upgraded"
	CatalogConditionDiskCached      condition.Cond = "DiskCached"
	CatalogConditionProcessed       condition.Cond = "Processed"
	CatalogConditionSecretsMigrated condition.Cond = "SecretsMigrated"
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

// Deprecated: CatalogStatus.HelmVersionCommits has been deprecated, which is the only consumer of this type.
type VersionCommits struct {
	Value map[string]string `json:"Value,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CatalogTemplate struct {
	types.Namespaced

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
	ProjectCatalogID         string `json:"projectCatalogId,omitempty" norman:"type=reference[projectCatalog]"`
	ClusterCatalogID         string `json:"clusterCatalogId,omitempty" norman:"type=reference[clusterCatalog]"`
	DefaultTemplateVersionID string `json:"defaultTemplateVersionId,omitempty" norman:"type=reference[templateVersion]"`
	ProjectID                string `json:"projectId,omitempty" norman:"required,type=reference[project]"`
	ClusterID                string `json:"clusterId,omitempty" norman:"required,type=reference[cluster]"`

	Description    string `json:"description,omitempty"`
	DefaultVersion string `json:"defaultVersion,omitempty" yaml:"default_version,omitempty"`
	Path           string `json:"path,omitempty"`
	Maintainer     string `json:"maintainer,omitempty"`
	ProjectURL     string `json:"projectURL,omitempty" yaml:"project_url,omitempty"`
	UpgradeFrom    string `json:"upgradeFrom,omitempty"`
	FolderName     string `json:"folderName,omitempty"`
	Icon           string `json:"icon,omitempty"`
	IconFilename   string `json:"iconFilename,omitempty"`

	// Deprecated: Do not use
	Readme string `json:"readme,omitempty" norman:"nocreate,noupdate"`

	Categories []string              `json:"categories,omitempty"`
	Versions   []TemplateVersionSpec `json:"versions,omitempty"`

	Category string `json:"category,omitempty"`
}

type TemplateStatus struct {
	HelmVersion string `json:"helmVersion,omitempty" norman:"noupdate,nocreate"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CatalogTemplateVersion struct {
	types.Namespaced
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
	RequiredNamespace   string            `json:"requiredNamespace,omitempty"`
	KubeVersion         string            `json:"kubeVersion,omitempty"`
	UpgradeVersionLinks map[string]string `json:"upgradeVersionLinks,omitempty"`
	Digest              string            `json:"digest,omitempty"`
	RancherMinVersion   string            `json:"rancherMinVersion,omitempty"`
	RancherMaxVersion   string            `json:"rancherMaxVersion,omitempty"`

	// Deprecated: Do not use
	Files map[string]string `json:"files,omitempty" norman:"nocreate,noupdate"`
	// Deprecated: Do not use
	Questions []Question `json:"questions,omitempty" norman:"nocreate,noupdate"`
	// Deprecated: Do not use
	Readme string `json:"readme,omitempty" norman:"nocreate,noupdate"`
	// Deprecated: Do not use
	AppReadme string `json:"appReadme,omitempty" norman:"nocreate,noupdate"`

	// for local cache rebuilt
	VersionName string   `json:"versionName,omitempty"`
	VersionDir  string   `json:"versionDir,omitempty"`
	VersionURLs []string `json:"versionUrls,omitempty"`
}

type TemplateVersionStatus struct {
	HelmVersion string `json:"helmVersion,omitempty" norman:"noupdate,nocreate"`
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
	Satisfies         string        `json:"satisfies,omitempty" yaml:"satisfies,omitempty"`
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
	Satisfies    string   `json:"satisfies,omitempty" yaml:"satisfies,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateContent is deprecated
//
// Deprecated: Do not use
type TemplateContent struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Data string `json:"data,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProjectCatalog struct {
	types.Namespaced

	Catalog     `json:",inline" mapstructure:",squash"`
	ProjectName string `json:"projectName,omitempty" norman:"type=reference[project]"`
}

func (p *ProjectCatalog) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterCatalog struct {
	types.Namespaced

	Catalog     `json:",inline" mapstructure:",squash"`
	ClusterName string `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`
}

type CatalogRefresh struct {
	Catalogs []string `json:"catalogs"`
}

type CatalogSecrets struct {
	CredentialSecret string `json:"credentialSecret,omitempty" norman:"nocreate,noupdate"`
}

// GetSecret gets a reference to the private catalog secret, either from the CatalogSecrets field or the Status field.
// Spec.CatalogSecrets.CredentialSecret is preferred because Status.CredentialSecret is deprecated.
func (c *Catalog) GetSecret() string {
	if c.Spec.CatalogSecrets.CredentialSecret != "" {
		return c.Spec.CatalogSecrets.CredentialSecret
	}
	return c.Status.CredentialSecret
}
