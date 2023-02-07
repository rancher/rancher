package registries

const RegistriesConfigKey = "registries"

type Registries struct {
	RegistryConfigNames []string `json:"registryConfigNames" yaml:"registryConfigNames" default:"[]"`
}
