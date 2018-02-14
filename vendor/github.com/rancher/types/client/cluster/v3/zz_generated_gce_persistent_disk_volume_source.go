package client

const (
	GCEPersistentDiskVolumeSourceType           = "gcePersistentDiskVolumeSource"
	GCEPersistentDiskVolumeSourceFieldFSType    = "fsType"
	GCEPersistentDiskVolumeSourceFieldPDName    = "pdName"
	GCEPersistentDiskVolumeSourceFieldPartition = "partition"
	GCEPersistentDiskVolumeSourceFieldReadOnly  = "readOnly"
)

type GCEPersistentDiskVolumeSource struct {
	FSType    string `json:"fsType,omitempty"`
	PDName    string `json:"pdName,omitempty"`
	Partition *int64 `json:"partition,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}
