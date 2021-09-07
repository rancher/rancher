package cloudcredentials

type S3CredentialConfig struct {
	AccessKey            string `json:"accessKey"`
	DefaultBucket        string `json:"defaultBucket,omitempty"`
	DefaultEndpoint      string `json:"defaultEndpoint,omitempty"`
	DefaultEndpointCA    string `json:"defaultEndpointCA,omitempty"`
	DefaultFolder        string `json:"defaultFolder,omitempty"`
	DefaultRegion        string `json:"defaultRegion,omitempty"`
	DefaultSkipSSLVerify string `json:"defaultSkipSSLVerify,omitempty"`
	SecretKey            string `json:"secretKey"`
}

type DigitalOceanCredentialConfig struct {
	AccessToken string `json:"accessToken,omitempty"`
}
