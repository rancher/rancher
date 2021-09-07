package client

const (
	HostAliasType           = "hostAlias"
	HostAliasFieldHostnames = "hostnames"
	HostAliasFieldIP        = "ip"
)

type HostAlias struct {
	Hostnames []string `json:"hostnames,omitempty" yaml:"hostnames,omitempty"`
	IP        string   `json:"ip,omitempty" yaml:"ip,omitempty"`
}
