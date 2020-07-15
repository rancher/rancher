package client

const (
	TemplateVersionSpecType                     = "templateVersionSpec"
	TemplateVersionSpecFieldAppReadme           = "appReadme"
	TemplateVersionSpecFieldDigest              = "digest"
	TemplateVersionSpecFieldExternalID          = "externalId"
	TemplateVersionSpecFieldFiles               = "files"
	TemplateVersionSpecFieldKubeVersion         = "kubeVersion"
	TemplateVersionSpecFieldQuestions           = "questions"
	TemplateVersionSpecFieldRancherMaxVersion   = "rancherMaxVersion"
	TemplateVersionSpecFieldRancherMinVersion   = "rancherMinVersion"
	TemplateVersionSpecFieldRancherVersion      = "rancherVersion"
	TemplateVersionSpecFieldReadme              = "readme"
	TemplateVersionSpecFieldRequiredNamespace   = "requiredNamespace"
	TemplateVersionSpecFieldUpgradeVersionLinks = "upgradeVersionLinks"
	TemplateVersionSpecFieldVersion             = "version"
	TemplateVersionSpecFieldVersionDir          = "versionDir"
	TemplateVersionSpecFieldVersionName         = "versionName"
	TemplateVersionSpecFieldVersionURLs         = "versionUrls"
)

type TemplateVersionSpec struct {
	AppReadme           string            `json:"appReadme,omitempty" yaml:"appReadme,omitempty"`
	Digest              string            `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID          string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files               map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	KubeVersion         string            `json:"kubeVersion,omitempty" yaml:"kubeVersion,omitempty"`
	Questions           []Question        `json:"questions,omitempty" yaml:"questions,omitempty"`
	RancherMaxVersion   string            `json:"rancherMaxVersion,omitempty" yaml:"rancherMaxVersion,omitempty"`
	RancherMinVersion   string            `json:"rancherMinVersion,omitempty" yaml:"rancherMinVersion,omitempty"`
	RancherVersion      string            `json:"rancherVersion,omitempty" yaml:"rancherVersion,omitempty"`
	Readme              string            `json:"readme,omitempty" yaml:"readme,omitempty"`
	RequiredNamespace   string            `json:"requiredNamespace,omitempty" yaml:"requiredNamespace,omitempty"`
	UpgradeVersionLinks map[string]string `json:"upgradeVersionLinks,omitempty" yaml:"upgradeVersionLinks,omitempty"`
	Version             string            `json:"version,omitempty" yaml:"version,omitempty"`
	VersionDir          string            `json:"versionDir,omitempty" yaml:"versionDir,omitempty"`
	VersionName         string            `json:"versionName,omitempty" yaml:"versionName,omitempty"`
	VersionURLs         []string          `json:"versionUrls,omitempty" yaml:"versionUrls,omitempty"`
}
