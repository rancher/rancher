package registries

const RegistriesConfigKey = "registries"

type ExistingAuthRegistryConfig struct {
	Username string `json:"username" yaml:"username" default:""`
	Password string `json:"password" yaml:"password" default:""`
	URL      string `json:"url" yaml:"url" default:""`
}

type Registries struct {
	RegistryConfigNames       []string                    `json:"registryConfigNames" yaml:"registryConfigNames" default:"[]"`
	ExistingNoAuthRegistryURL string                      `json:"existingNoAuthRegistry" yaml:"existingNoAuthRegistry" default:""`
	ExistingAuthRegistryInfo  *ExistingAuthRegistryConfig `json:"existingAuthRegistry" yaml:"existingAuthRegistry" default:""`
}
