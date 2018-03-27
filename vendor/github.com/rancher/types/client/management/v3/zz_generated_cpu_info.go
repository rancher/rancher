package client

const (
	CPUInfoType       = "cpuInfo"
	CPUInfoFieldCount = "count"
)

type CPUInfo struct {
	Count int64 `json:"count,omitempty" yaml:"count,omitempty"`
}
