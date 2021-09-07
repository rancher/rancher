package client

const (
	VsphereVirtualDiskVolumeSourceType                   = "vsphereVirtualDiskVolumeSource"
	VsphereVirtualDiskVolumeSourceFieldFSType            = "fsType"
	VsphereVirtualDiskVolumeSourceFieldStoragePolicyID   = "storagePolicyID"
	VsphereVirtualDiskVolumeSourceFieldStoragePolicyName = "storagePolicyName"
	VsphereVirtualDiskVolumeSourceFieldVolumePath        = "volumePath"
)

type VsphereVirtualDiskVolumeSource struct {
	FSType            string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	StoragePolicyID   string `json:"storagePolicyID,omitempty" yaml:"storagePolicyID,omitempty"`
	StoragePolicyName string `json:"storagePolicyName,omitempty" yaml:"storagePolicyName,omitempty"`
	VolumePath        string `json:"volumePath,omitempty" yaml:"volumePath,omitempty"`
}
