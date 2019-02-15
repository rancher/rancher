package client

const (
	AppUpgradeConfigType              = "appUpgradeConfig"
	AppUpgradeConfigFieldAnswers      = "answers"
	AppUpgradeConfigFieldExternalID   = "externalId"
	AppUpgradeConfigFieldFiles        = "files"
	AppUpgradeConfigFieldForceUpgrade = "forceUpgrade"
	AppUpgradeConfigFieldValuesYaml   = "valuesYaml"
)

type AppUpgradeConfig struct {
	Answers      map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	ExternalID   string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files        map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	ForceUpgrade bool              `json:"forceUpgrade,omitempty" yaml:"forceUpgrade,omitempty"`
	ValuesYaml   string            `json:"valuesYaml,omitempty" yaml:"valuesYaml,omitempty"`
}
