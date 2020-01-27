package client

const (
	NodeUpgradeStatusType                  = "nodeUpgradeStatus"
	NodeUpgradeStatusFieldCurrentToken     = "currentToken"
	NodeUpgradeStatusFieldLastAppliedToken = "lastAppliedToken"
	NodeUpgradeStatusFieldNodes            = "nodes"
)

type NodeUpgradeStatus struct {
	CurrentToken     string                       `json:"currentToken,omitempty" yaml:"currentToken,omitempty"`
	LastAppliedToken string                       `json:"lastAppliedToken,omitempty" yaml:"lastAppliedToken,omitempty"`
	Nodes            map[string]map[string]string `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}
