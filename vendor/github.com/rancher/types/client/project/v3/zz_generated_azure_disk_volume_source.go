package client

const (
	AzureDiskVolumeSourceType             = "azureDiskVolumeSource"
	AzureDiskVolumeSourceFieldCachingMode = "cachingMode"
	AzureDiskVolumeSourceFieldDataDiskURI = "diskURI"
	AzureDiskVolumeSourceFieldDiskName    = "diskName"
	AzureDiskVolumeSourceFieldFSType      = "fsType"
	AzureDiskVolumeSourceFieldKind        = "kind"
	AzureDiskVolumeSourceFieldReadOnly    = "readOnly"
)

type AzureDiskVolumeSource struct {
	CachingMode string `json:"cachingMode,omitempty"`
	DataDiskURI string `json:"diskURI,omitempty"`
	DiskName    string `json:"diskName,omitempty"`
	FSType      string `json:"fsType,omitempty"`
	Kind        string `json:"kind,omitempty"`
	ReadOnly    *bool  `json:"readOnly,omitempty"`
}
