package client

const (
	TemplateVersionSpecType                     = "templateVersionSpec"
	TemplateVersionSpecFieldAppReadme           = "appReadme"
	TemplateVersionSpecFieldDigest              = "digest"
	TemplateVersionSpecFieldExternalID          = "externalId"
	TemplateVersionSpecFieldFiles               = "files"
	TemplateVersionSpecFieldKubeVersion         = "kubeVersion"
	TemplateVersionSpecFieldQuestions           = "questions"
	TemplateVersionSpecFieldRancherVersion      = "rancherVersion"
	TemplateVersionSpecFieldReadme              = "readme"
	TemplateVersionSpecFieldRequiredNamespace   = "requiredNamespace"
	TemplateVersionSpecFieldUpgradeVersionLinks = "upgradeVersionLinks"
	TemplateVersionSpecFieldVersion             = "version"
)

type TemplateVersionSpec struct {
	AppReadme           string            `json:"appReadme,omitempty" yaml:"appReadme,omitempty"`
	Digest              string            `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID          string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files               map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	KubeVersion         string            `json:"kubeVersion,omitempty" yaml:"kubeVersion,omitempty"`
	Questions           []Question        `json:"questions,omitempty" yaml:"questions,omitempty"`
	RancherVersion      string            `json:"rancherVersion,omitempty" yaml:"rancherVersion,omitempty"`
	Readme              string            `json:"readme,omitempty" yaml:"readme,omitempty"`
	RequiredNamespace   string            `json:"requiredNamespace,omitempty" yaml:"requiredNamespace,omitempty"`
	UpgradeVersionLinks map[string]string `json:"upgradeVersionLinks,omitempty" yaml:"upgradeVersionLinks,omitempty"`
	Version             string            `json:"version,omitempty" yaml:"version,omitempty"`
}
