package rancher

const ConfigurationFileKey = "rancher"

type Config struct {
	Host       string `yaml:"host"`
	AdminToken string `yaml:"adminToken"`
	Insecure   *bool  `yaml:"insecure" default:"true"`
	Cleanup    *bool  `yaml:"cleanup" default:"true"`
	CAFile     string `yaml:"caFile" default:""`
	CACerts    string `yaml:"caCerts" default:""`
}
