package client

const (
	TemplateVersionSpecType                       = "templateVersionSpec"
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
	Files                 []File            `json:"files,omitempty"`
	MaximumRancherVersion string            `json:"maximumRancherVersion,omitempty"`
	MinimumRancherVersion string            `json:"minimumRancherVersion,omitempty"`
	Questions             []Question        `json:"questions,omitempty"`
	Readme                string            `json:"readme,omitempty"`
	Revision              *int64            `json:"revision,omitempty"`
	UpgradeFrom           string            `json:"upgradeFrom,omitempty"`
	UpgradeVersionLinks   map[string]string `json:"upgradeVersionLinks,omitempty"`
	Version               string            `json:"version,omitempty"`
}
