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
	CachingMode string `json:"cachingMode,omitempty" yaml:"cachingMode,omitempty"`
	DataDiskURI string `json:"diskURI,omitempty" yaml:"diskURI,omitempty"`
	DiskName    string `json:"diskName,omitempty" yaml:"diskName,omitempty"`
	FSType      string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Kind        string `json:"kind,omitempty" yaml:"kind,omitempty"`
	ReadOnly    *bool  `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
