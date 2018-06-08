package client

const (
	RouterOpenstackOptsType          = "routerOpenstackOpts"
	RouterOpenstackOptsFieldRouterID = "router-id"
)

type RouterOpenstackOpts struct {
	RouterID string `json:"router-id,omitempty" yaml:"router-id,omitempty"`
}
