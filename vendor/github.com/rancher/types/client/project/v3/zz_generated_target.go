package client

const (
	TargetType                   = "target"
	TargetFieldAddresses         = "addresses"
	TargetFieldNotReadyAddresses = "notReadyAddresses"
	TargetFieldPort              = "port"
	TargetFieldProtocol          = "protocol"
)

type Target struct {
	Addresses         []string `json:"addresses,omitempty"`
	NotReadyAddresses []string `json:"notReadyAddresses,omitempty"`
	Port              *int64   `json:"port,omitempty"`
	Protocol          string   `json:"protocol,omitempty"`
}
