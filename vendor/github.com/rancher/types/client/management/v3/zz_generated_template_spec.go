package client

const (
	TemplateSpecType                          = "templateSpec"
	TemplateSpecFieldBase                     = "templateBase"
	TemplateSpecFieldCatalogID                = "catalogId"
	TemplateSpecFieldCategories               = "categories"
	TemplateSpecFieldCategory                 = "category"
	TemplateSpecFieldDefaultTemplateVersionID = "defaultTemplateVersionId"
	TemplateSpecFieldDefaultVersion           = "defaultVersion"
	TemplateSpecFieldDescription              = "description"
	TemplateSpecFieldFolderName               = "folderName"
	TemplateSpecFieldIcon                     = "icon"
	TemplateSpecFieldIconFilename             = "iconFilename"
	TemplateSpecFieldIsSystem                 = "isSystem"
	TemplateSpecFieldLicense                  = "license"
	TemplateSpecFieldMaintainer               = "maintainer"
	TemplateSpecFieldPath                     = "path"
	TemplateSpecFieldProjectURL               = "projectURL"
	TemplateSpecFieldReadme                   = "readme"
	TemplateSpecFieldUpgradeFrom              = "upgradeFrom"
	TemplateSpecFieldVersions                 = "versions"
)

type TemplateSpec struct {
	Base                     string                `json:"templateBase,omitempty"`
	CatalogID                string                `json:"catalogId,omitempty"`
	Categories               []string              `json:"categories,omitempty"`
	Category                 string                `json:"category,omitempty"`
	DefaultTemplateVersionID string                `json:"defaultTemplateVersionId,omitempty"`
	DefaultVersion           string                `json:"defaultVersion,omitempty"`
	Description              string                `json:"description,omitempty"`
	FolderName               string                `json:"folderName,omitempty"`
	Icon                     string                `json:"icon,omitempty"`
	IconFilename             string                `json:"iconFilename,omitempty"`
	IsSystem                 string                `json:"isSystem,omitempty"`
	License                  string                `json:"license,omitempty"`
	Maintainer               string                `json:"maintainer,omitempty"`
	Path                     string                `json:"path,omitempty"`
	ProjectURL               string                `json:"projectURL,omitempty"`
	Readme                   string                `json:"readme,omitempty"`
	UpgradeFrom              string                `json:"upgradeFrom,omitempty"`
	Versions                 []TemplateVersionSpec `json:"versions,omitempty"`
}
