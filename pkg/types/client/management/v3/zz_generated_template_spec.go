package client

const (
	TemplateSpecType                          = "templateSpec"
	TemplateSpecFieldCatalogID                = "catalogId"
	TemplateSpecFieldCategories               = "categories"
	TemplateSpecFieldCategory                 = "category"
	TemplateSpecFieldClusterCatalogID         = "clusterCatalogId"
	TemplateSpecFieldClusterID                = "clusterId"
	TemplateSpecFieldDefaultTemplateVersionID = "defaultTemplateVersionId"
	TemplateSpecFieldDefaultVersion           = "defaultVersion"
	TemplateSpecFieldDescription              = "description"
	TemplateSpecFieldDisplayName              = "displayName"
	TemplateSpecFieldFolderName               = "folderName"
	TemplateSpecFieldIcon                     = "icon"
	TemplateSpecFieldIconFilename             = "iconFilename"
	TemplateSpecFieldMaintainer               = "maintainer"
	TemplateSpecFieldPath                     = "path"
	TemplateSpecFieldProjectCatalogID         = "projectCatalogId"
	TemplateSpecFieldProjectID                = "projectId"
	TemplateSpecFieldProjectURL               = "projectURL"
	TemplateSpecFieldReadme                   = "readme"
	TemplateSpecFieldUpgradeFrom              = "upgradeFrom"
	TemplateSpecFieldVersions                 = "versions"
)

type TemplateSpec struct {
	CatalogID                string                `json:"catalogId,omitempty" yaml:"catalogId,omitempty"`
	Categories               []string              `json:"categories,omitempty" yaml:"categories,omitempty"`
	Category                 string                `json:"category,omitempty" yaml:"category,omitempty"`
	ClusterCatalogID         string                `json:"clusterCatalogId,omitempty" yaml:"clusterCatalogId,omitempty"`
	ClusterID                string                `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	DefaultTemplateVersionID string                `json:"defaultTemplateVersionId,omitempty" yaml:"defaultTemplateVersionId,omitempty"`
	DefaultVersion           string                `json:"defaultVersion,omitempty" yaml:"defaultVersion,omitempty"`
	Description              string                `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName              string                `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	FolderName               string                `json:"folderName,omitempty" yaml:"folderName,omitempty"`
	Icon                     string                `json:"icon,omitempty" yaml:"icon,omitempty"`
	IconFilename             string                `json:"iconFilename,omitempty" yaml:"iconFilename,omitempty"`
	Maintainer               string                `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	Path                     string                `json:"path,omitempty" yaml:"path,omitempty"`
	ProjectCatalogID         string                `json:"projectCatalogId,omitempty" yaml:"projectCatalogId,omitempty"`
	ProjectID                string                `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	ProjectURL               string                `json:"projectURL,omitempty" yaml:"projectURL,omitempty"`
	Readme                   string                `json:"readme,omitempty" yaml:"readme,omitempty"`
	UpgradeFrom              string                `json:"upgradeFrom,omitempty" yaml:"upgradeFrom,omitempty"`
	Versions                 []TemplateVersionSpec `json:"versions,omitempty" yaml:"versions,omitempty"`
}
