package client

const (
	PhotonPersistentDiskVolumeSourceType        = "photonPersistentDiskVolumeSource"
	PhotonPersistentDiskVolumeSourceFieldFSType = "fsType"
	PhotonPersistentDiskVolumeSourceFieldPdID   = "pdID"
)

type PhotonPersistentDiskVolumeSource struct {
	FSType string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	PdID   string `json:"pdID,omitempty" yaml:"pdID,omitempty"`
}
