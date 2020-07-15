package client

const (
	OpenstackCloudProviderType              = "openstackCloudProvider"
	OpenstackCloudProviderFieldBlockStorage = "blockStorage"
	OpenstackCloudProviderFieldGlobal       = "global"
	OpenstackCloudProviderFieldLoadBalancer = "loadBalancer"
	OpenstackCloudProviderFieldMetadata     = "metadata"
	OpenstackCloudProviderFieldRoute        = "route"
)

type OpenstackCloudProvider struct {
	BlockStorage *BlockStorageOpenstackOpts `json:"blockStorage,omitempty" yaml:"blockStorage,omitempty"`
	Global       *GlobalOpenstackOpts       `json:"global,omitempty" yaml:"global,omitempty"`
	LoadBalancer *LoadBalancerOpenstackOpts `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
	Metadata     *MetadataOpenstackOpts     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Route        *RouteOpenstackOpts        `json:"route,omitempty" yaml:"route,omitempty"`
}
