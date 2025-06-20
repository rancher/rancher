package client

const (
	NodeSwapStatusType          = "nodeSwapStatus"
	NodeSwapStatusFieldCapacity = "capacity"
)

type NodeSwapStatus struct {
	Capacity *int64 `json:"capacity,omitempty" yaml:"capacity,omitempty"`
}
