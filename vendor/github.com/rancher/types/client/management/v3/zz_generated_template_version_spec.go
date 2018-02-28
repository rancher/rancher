package client

const (
	TemplateVersionSpecType                       = "templateVersionSpec"
	TemplateVersionSpecFieldExternalID            = "externalId"
	TemplateVersionSpecFieldFiles                 = "files"
	TemplateVersionSpecFieldMaximumRancherVersion = "maximumRancherVersion"
	TemplateVersionSpecFieldMinimumRancherVersion = "minimumRancherVersion"
	TemplateVersionSpecFieldQuestions             = "questions"
	TemplateVersionSpecFieldReadme                = "readme"
	TemplateVersionSpecFieldRevision              = "revision"
	TemplateVersionSpecFieldUpgradeFrom           = "upgradeFrom"
	TemplateVersionSpecFieldUpgradeVersionLinks   = "upgradeVersionLinks"
	TemplateVersionSpecFieldVersion               = "version"
)

type TemplateVersionSpec struct {
	ExternalID            string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files                 []File            `json:"files,omitempty" yaml:"files,omitempty"`
	MaximumRancherVersion string            `json:"maximumRancherVersion,omitempty" yaml:"maximumRancherVersion,omitempty"`
	MinimumRancherVersion string            `json:"minimumRancherVersion,omitempty" yaml:"minimumRancherVersion,omitempty"`
	Questions             []Question        `json:"questions,omitempty" yaml:"questions,omitempty"`
	Readme                string            `json:"readme,omitempty" yaml:"readme,omitempty"`
	Revision              *int64            `json:"revision,omitempty" yaml:"revision,omitempty"`
	UpgradeFrom           string            `json:"upgradeFrom,omitempty" yaml:"upgradeFrom,omitempty"`
	UpgradeVersionLinks   map[string]string `json:"upgradeVersionLinks,omitempty" yaml:"upgradeVersionLinks,omitempty"`
	Version               string            `json:"version,omitempty" yaml:"version,omitempty"`
}
