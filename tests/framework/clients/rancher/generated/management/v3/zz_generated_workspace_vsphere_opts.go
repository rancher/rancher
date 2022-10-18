package client

const (
	WorkspaceVsphereOptsType                  = "workspaceVsphereOpts"
	WorkspaceVsphereOptsFieldDatacenter       = "datacenter"
	WorkspaceVsphereOptsFieldDefaultDatastore = "default-datastore"
	WorkspaceVsphereOptsFieldFolder           = "folder"
	WorkspaceVsphereOptsFieldResourcePoolPath = "resourcepool-path"
	WorkspaceVsphereOptsFieldVCenterIP        = "server"
)

type WorkspaceVsphereOpts struct {
	Datacenter       string `json:"datacenter,omitempty" yaml:"datacenter,omitempty"`
	DefaultDatastore string `json:"default-datastore,omitempty" yaml:"default-datastore,omitempty"`
	Folder           string `json:"folder,omitempty" yaml:"folder,omitempty"`
	ResourcePoolPath string `json:"resourcepool-path,omitempty" yaml:"resourcepool-path,omitempty"`
	VCenterIP        string `json:"server,omitempty" yaml:"server,omitempty"`
}
