package rancher

const ConfigurationFileKey = "rancher"

type Config struct {
	RancherHost string `yaml:"rancherHost"`
	AdminToken  string `yaml:"adminToken"`
	Insecure    *bool  `yaml:"insecure" default:"true"`
	CAFile      string `yaml:"caFile" default:""`
	CACerts     string `yaml:"caCerts" default:""`
}
