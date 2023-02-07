package registries

const RegistriesConfigKey = "registries"

type Config struct {
	Clusters []Cluster `json:"clusters" yaml:"clusters" default:"[]"`
}

type Cluster struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url" yaml:"url"`
	Auth bool   `json:"auth" yaml:"auth"`
}
