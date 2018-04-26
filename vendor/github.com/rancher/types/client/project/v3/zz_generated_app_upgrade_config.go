package client

const (
	AppUpgradeConfigType            = "appUpgradeConfig"
	AppUpgradeConfigFieldAnswers    = "answers"
	AppUpgradeConfigFieldExternalID = "externalId"
)

type AppUpgradeConfig struct {
	Answers    map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	ExternalID string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
}
