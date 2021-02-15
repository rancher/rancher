package client

const (
	AppUpgradeConfigType                    = "appUpgradeConfig"
	AppUpgradeConfigFieldAnswers            = "answers"
	AppUpgradeConfigFieldAnswersForceString = "answersForceString"
	AppUpgradeConfigFieldExternalID         = "externalId"
	AppUpgradeConfigFieldFiles              = "files"
	AppUpgradeConfigFieldForceUpgrade       = "forceUpgrade"
	AppUpgradeConfigFieldValuesYaml         = "valuesYaml"
)

type AppUpgradeConfig struct {
	Answers            map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersForceString map[string]bool   `json:"answersForceString,omitempty" yaml:"answersForceString,omitempty"`
	ExternalID         string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files              map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	ForceUpgrade       bool              `json:"forceUpgrade,omitempty" yaml:"forceUpgrade,omitempty"`
	ValuesYaml         string            `json:"valuesYaml,omitempty" yaml:"valuesYaml,omitempty"`
}
