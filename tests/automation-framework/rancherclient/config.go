package rancherclient

const ConfigurationFileKey = "rancher"

type Config struct {
	RancherHost string `json:"rancherHost"`
	AdminToken  string `json:"adminToken"`
	Insecure    *bool  `json:"insecure" default:"true"`
	CAFile      string `json:"caFile" default:""`
	CACerts     string `json:"caCerts" default:""`
}
