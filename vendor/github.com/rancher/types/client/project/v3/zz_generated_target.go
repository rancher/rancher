package client

const (
	TargetType           = "target"
	TargetFieldAddresses = "addresses"
	TargetFieldPort      = "port"
	TargetFieldProtocol  = "protocol"
)

type Target struct {
	Addresses []string `json:"addresses,omitempty"`
	Port      *int64   `json:"port,omitempty"`
	Protocol  string   `json:"protocol,omitempty"`
}
