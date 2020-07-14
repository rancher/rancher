package client

const (
	Route53ProviderConfigType                 = "route53ProviderConfig"
	Route53ProviderConfigFieldAccessKey       = "accessKey"
	Route53ProviderConfigFieldCredentialsPath = "credentialsPath"
	Route53ProviderConfigFieldRegion          = "region"
	Route53ProviderConfigFieldRoleArn         = "roleArn"
	Route53ProviderConfigFieldSecretKey       = "secretKey"
	Route53ProviderConfigFieldZoneType        = "zoneType"
)

type Route53ProviderConfig struct {
	AccessKey       string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	CredentialsPath string `json:"credentialsPath,omitempty" yaml:"credentialsPath,omitempty"`
	Region          string `json:"region,omitempty" yaml:"region,omitempty"`
	RoleArn         string `json:"roleArn,omitempty" yaml:"roleArn,omitempty"`
	SecretKey       string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
	ZoneType        string `json:"zoneType,omitempty" yaml:"zoneType,omitempty"`
}
