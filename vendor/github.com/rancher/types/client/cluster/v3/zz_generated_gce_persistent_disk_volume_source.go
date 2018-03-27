package client

const (
	GCEPersistentDiskVolumeSourceType           = "gcePersistentDiskVolumeSource"
	GCEPersistentDiskVolumeSourceFieldFSType    = "fsType"
	GCEPersistentDiskVolumeSourceFieldPDName    = "pdName"
	GCEPersistentDiskVolumeSourceFieldPartition = "partition"
	GCEPersistentDiskVolumeSourceFieldReadOnly  = "readOnly"
)

type GCEPersistentDiskVolumeSource struct {
	FSType    string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	PDName    string `json:"pdName,omitempty" yaml:"pdName,omitempty"`
	Partition int64  `json:"partition,omitempty" yaml:"partition,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
