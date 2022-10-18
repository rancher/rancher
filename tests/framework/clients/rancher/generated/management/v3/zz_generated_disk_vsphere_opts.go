package client

const (
	DiskVsphereOptsType                    = "diskVsphereOpts"
	DiskVsphereOptsFieldSCSIControllerType = "scsicontrollertype"
)

type DiskVsphereOpts struct {
	SCSIControllerType string `json:"scsicontrollertype,omitempty" yaml:"scsicontrollertype,omitempty"`
}
