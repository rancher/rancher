package client

const (
	GlobalVsphereOptsType                   = "globalVsphereOpts"
	GlobalVsphereOptsFieldDatacenter        = "datacenter"
	GlobalVsphereOptsFieldDatacenters       = "datacenters"
	GlobalVsphereOptsFieldDefaultDatastore  = "datastore"
	GlobalVsphereOptsFieldInsecureFlag      = "insecure-flag"
	GlobalVsphereOptsFieldPassword          = "password"
	GlobalVsphereOptsFieldRoundTripperCount = "soap-roundtrip-count"
	GlobalVsphereOptsFieldUser              = "user"
	GlobalVsphereOptsFieldVCenterIP         = "server"
	GlobalVsphereOptsFieldVCenterPort       = "port"
	GlobalVsphereOptsFieldVMName            = "vm-name"
	GlobalVsphereOptsFieldVMUUID            = "vm-uuid"
	GlobalVsphereOptsFieldWorkingDir        = "working-dir"
)

type GlobalVsphereOpts struct {
	Datacenter        string `json:"datacenter,omitempty" yaml:"datacenter,omitempty"`
	Datacenters       string `json:"datacenters,omitempty" yaml:"datacenters,omitempty"`
	DefaultDatastore  string `json:"datastore,omitempty" yaml:"datastore,omitempty"`
	InsecureFlag      bool   `json:"insecure-flag,omitempty" yaml:"insecure-flag,omitempty"`
	Password          string `json:"password,omitempty" yaml:"password,omitempty"`
	RoundTripperCount int64  `json:"soap-roundtrip-count,omitempty" yaml:"soap-roundtrip-count,omitempty"`
	User              string `json:"user,omitempty" yaml:"user,omitempty"`
	VCenterIP         string `json:"server,omitempty" yaml:"server,omitempty"`
	VCenterPort       string `json:"port,omitempty" yaml:"port,omitempty"`
	VMName            string `json:"vm-name,omitempty" yaml:"vm-name,omitempty"`
	VMUUID            string `json:"vm-uuid,omitempty" yaml:"vm-uuid,omitempty"`
	WorkingDir        string `json:"working-dir,omitempty" yaml:"working-dir,omitempty"`
}
