package client

const (
	TemplateVersionSpecType                       = "templateVersionSpec"
	TemplateVersionSpecFieldAppReadme             = "appReadme"
	TemplateVersionSpecFieldDigest                = "digest"
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
	AppReadme             string            `json:"appReadme,omitempty" yaml:"appReadme,omitempty"`
	Digest                string            `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID            string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files                 map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	MaximumRancherVersion string            `json:"maximumRancherVersion,omitempty" yaml:"maximumRancherVersion,omitempty"`
	MinimumRancherVersion string            `json:"minimumRancherVersion,omitempty" yaml:"minimumRancherVersion,omitempty"`
	Questions             []Question        `json:"questions,omitempty" yaml:"questions,omitempty"`
	Readme                string            `json:"readme,omitempty" yaml:"readme,omitempty"`
	Revision              *int64            `json:"revision,omitempty" yaml:"revision,omitempty"`
	UpgradeFrom           string            `json:"upgradeFrom,omitempty" yaml:"upgradeFrom,omitempty"`
	UpgradeVersionLinks   map[string]string `json:"upgradeVersionLinks,omitempty" yaml:"upgradeVersionLinks,omitempty"`
	Version               string            `json:"version,omitempty" yaml:"version,omitempty"`
}
