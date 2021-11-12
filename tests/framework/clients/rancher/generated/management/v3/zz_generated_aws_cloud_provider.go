package client

const (
	AWSCloudProviderType                 = "awsCloudProvider"
	AWSCloudProviderFieldGlobal          = "global"
	AWSCloudProviderFieldServiceOverride = "serviceOverride"
)

type AWSCloudProvider struct {
	Global          *GlobalAwsOpts             `json:"global,omitempty" yaml:"global,omitempty"`
	ServiceOverride map[string]ServiceOverride `json:"serviceOverride,omitempty" yaml:"serviceOverride,omitempty"`
}
