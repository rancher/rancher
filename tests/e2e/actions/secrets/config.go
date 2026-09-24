package secrets

const (
	ConfigurationFileKey = "registryInput"
)

type Config struct {
	Name     string `yaml:"name" json:"name" default:"quay.io"`
	Username string `yaml:"username" json:"username" default:""`
	Password string `yaml:"password" json:"password" default:""`
}
