package client

const (
	LinuxContainerUserType                    = "linuxContainerUser"
	LinuxContainerUserFieldGID                = "gid"
	LinuxContainerUserFieldSupplementalGroups = "supplementalGroups"
	LinuxContainerUserFieldUID                = "uid"
)

type LinuxContainerUser struct {
	GID                int64   `json:"gid,omitempty" yaml:"gid,omitempty"`
	SupplementalGroups []int64 `json:"supplementalGroups,omitempty" yaml:"supplementalGroups,omitempty"`
	UID                int64   `json:"uid,omitempty" yaml:"uid,omitempty"`
}
