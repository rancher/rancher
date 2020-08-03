package client

const (
	RecipientType           = "recipient"
	RecipientFieldNotifier  = "notifier"
	RecipientFieldRecipient = "recipient"
)

type Recipient struct {
	Notifier  string `json:"notifier,omitempty" yaml:"notifier,omitempty"`
	Recipient string `json:"recipient,omitempty" yaml:"recipient,omitempty"`
}
