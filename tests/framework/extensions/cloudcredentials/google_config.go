package cloudcredentials

// The json/yaml config key for the google cloud credential config
const GoogleCredentialConfigurationFileKey = "googleCredentials"

// GoogleCredentialConfig is configuration need to create a google cloud credential
type GoogleCredentialConfig struct {
	AuthEncodedJSON string `json:"authEncodedJson" yaml:"authEncodedJson"`
}
