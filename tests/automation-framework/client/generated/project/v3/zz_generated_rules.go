package client

const (
	RulesType       = "rules"
	RulesFieldAlert = "alert"
)

type Rules struct {
	Alert *RulesAlert `json:"alert,omitempty" yaml:"alert,omitempty"`
}
