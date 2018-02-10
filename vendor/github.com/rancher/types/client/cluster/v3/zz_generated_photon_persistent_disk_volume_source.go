package client

const (
	PhotonPersistentDiskVolumeSourceType        = "photonPersistentDiskVolumeSource"
	PhotonPersistentDiskVolumeSourceFieldFSType = "fsType"
	PhotonPersistentDiskVolumeSourceFieldPdID   = "pdID"
)

type PhotonPersistentDiskVolumeSource struct {
	FSType string `json:"fsType,omitempty"`
	PdID   string `json:"pdID,omitempty"`
}
