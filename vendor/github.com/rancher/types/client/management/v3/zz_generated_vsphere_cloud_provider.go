package client

const (
	VsphereCloudProviderType               = "vsphereCloudProvider"
	VsphereCloudProviderFieldDisk          = "disk"
	VsphereCloudProviderFieldGlobal        = "global"
	VsphereCloudProviderFieldNetwork       = "network"
	VsphereCloudProviderFieldVirtualCenter = "virtualCenter"
	VsphereCloudProviderFieldWorkspace     = "workspace"
)

type VsphereCloudProvider struct {
	Disk          *DiskVsphereOpts               `json:"disk,omitempty" yaml:"disk,omitempty"`
	Global        *GlobalVsphereOpts             `json:"global,omitempty" yaml:"global,omitempty"`
	Network       *NetworkVshpereOpts            `json:"network,omitempty" yaml:"network,omitempty"`
	VirtualCenter map[string]VirtualCenterConfig `json:"virtualCenter,omitempty" yaml:"virtualCenter,omitempty"`
	Workspace     *WorkspaceVsphereOpts          `json:"workspace,omitempty" yaml:"workspace,omitempty"`
}
