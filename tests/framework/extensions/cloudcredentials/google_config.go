package cloudcredentials

const GoogleCredentialConfigurationFileKey = "googleCredentials"

type GoogleCredentialConfig struct {
	AuthEncodedJson string `json:"authEncodedJson" yaml:"authEncodedJson"`
}
