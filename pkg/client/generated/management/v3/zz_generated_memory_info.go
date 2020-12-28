package client

const (
	MemoryInfoType             = "memoryInfo"
	MemoryInfoFieldMemTotalKiB = "memTotalKiB"
)

type MemoryInfo struct {
	MemTotalKiB int64 `json:"memTotalKiB,omitempty" yaml:"memTotalKiB,omitempty"`
}
