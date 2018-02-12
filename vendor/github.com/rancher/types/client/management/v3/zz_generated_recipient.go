package client

const (
	RecipientType              = "recipient"
	RecipientFieldNotifierId   = "notifierId"
	RecipientFieldNotifierType = "notifierType"
	RecipientFieldRecipient    = "recipient"
)

type Recipient struct {
	NotifierId   string `json:"notifierId,omitempty"`
	NotifierType string `json:"notifierType,omitempty"`
	Recipient    string `json:"recipient,omitempty"`
}
