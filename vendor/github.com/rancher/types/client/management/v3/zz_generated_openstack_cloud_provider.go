package client

const (
	OpenstackCloudProviderType              = "openstackCloudProvider"
	OpenstackCloudProviderFieldBlockStorage = "blockStorage"
	OpenstackCloudProviderFieldGlobal       = "global"
	OpenstackCloudProviderFieldLoadBalancer = "loadBalancer"
	OpenstackCloudProviderFieldMetadata     = "metadata"
	OpenstackCloudProviderFieldRouter       = "router"
)

type OpenstackCloudProvider struct {
	BlockStorage *BlockStorageOpenstackOpts `json:"blockStorage,omitempty" yaml:"blockStorage,omitempty"`
	Global       *GlobalOpenstackOpts       `json:"global,omitempty" yaml:"global,omitempty"`
	LoadBalancer *LoadBalancerOpenstackOpts `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
	Metadata     *MetadataOpenstackOpts     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Router       *RouterOpenstackOpts       `json:"router,omitempty" yaml:"router,omitempty"`
}
