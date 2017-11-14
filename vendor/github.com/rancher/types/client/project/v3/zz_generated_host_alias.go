package client

const (
	HostAliasType           = "hostAlias"
	HostAliasFieldHostnames = "hostnames"
)

type HostAlias struct {
	Hostnames []string `json:"hostnames,omitempty"`
}
