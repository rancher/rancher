package client

const (
	SlackConfigType                  = "slackConfig"
	SlackConfigFieldDefaultRecipient = "defaultRecipient"
	SlackConfigFieldURL              = "url"
)

type SlackConfig struct {
	DefaultRecipient string `json:"defaultRecipient,omitempty"`
	URL              string `json:"url,omitempty"`
}
