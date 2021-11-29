package k3d

const ConfigurationFileKey = "k3d"

type Config struct {
	image         string `yaml:"image" default:"rancher/k3s:v1.21.3-k3s1"`
	createTimeout int    `yaml:"createTimeout" default:"120s"`
}
