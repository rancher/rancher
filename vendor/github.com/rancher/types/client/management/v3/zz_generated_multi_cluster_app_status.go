package client

const (
	MultiClusterAppStatusType             = "multiClusterAppStatus"
	MultiClusterAppStatusFieldHealthstate = "healthState"
)

type MultiClusterAppStatus struct {
	Healthstate string `json:"healthState,omitempty" yaml:"healthState,omitempty"`
}
