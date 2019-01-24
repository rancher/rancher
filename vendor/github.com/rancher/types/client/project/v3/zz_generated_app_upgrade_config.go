package client

const (
	AppUpgradeConfigType              = "appUpgradeConfig"
	AppUpgradeConfigFieldAnswers      = "answers"
	AppUpgradeConfigFieldExternalID   = "externalId"
	AppUpgradeConfigFieldForceUpgrade = "forceUpgrade"
)

type AppUpgradeConfig struct {
	Answers      map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	ExternalID   string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	ForceUpgrade bool              `json:"forceUpgrade,omitempty" yaml:"forceUpgrade,omitempty"`
}
