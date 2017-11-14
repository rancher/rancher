package client

const (
	HostPortRangeType     = "hostPortRange"
	HostPortRangeFieldMax = "max"
	HostPortRangeFieldMin = "min"
)

type HostPortRange struct {
	Max *int64 `json:"max,omitempty"`
	Min *int64 `json:"min,omitempty"`
}
