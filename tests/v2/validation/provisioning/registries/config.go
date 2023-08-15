package registries

const RegistriesConfigKey = "registries"

type ExistingAuthRegistryConfig struct {
	Username string `json:"username" yaml:"username" default:""`
	Password string `json:"password" yaml:"password" default:""`
	URL      string `json:"url" yaml:"url" default:""`
}

type ECRRegistryConfig struct {
	Username           string `json:"username" yaml:"username" default:"AWS"`
	Password           string `json:"password" yaml:"password" default:""`
	URL                string `json:"url" yaml:"url" default:""`
	AwsAccessKeyID     string `json:"awsAccessKeyId" yaml:"awsAccessKeyId" default:""`
	AwsSecretAccessKey string `json:"awsSecretAccessKey" yaml:"awsSecretAccessKey" default:""`
}

type Registries struct {
	RegistryConfigNames       []string                    `json:"registryConfigNames" yaml:"registryConfigNames" default:"[]"`
	ExistingNoAuthRegistryURL string                      `json:"existingNoAuthRegistry" yaml:"existingNoAuthRegistry" default:""`
	ExistingAuthRegistryInfo  *ExistingAuthRegistryConfig `json:"existingAuthRegistry" yaml:"existingAuthRegistry" default:""`
	ECRRegistryConfig         *ECRRegistryConfig          `json:"ecrRegistryConfig" yaml:"ecrRegistryConfig" default:""`
}
