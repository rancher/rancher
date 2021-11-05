package rancherclient

type Config struct {
	RancherHost       string `json:"rancherHost"`
	AdminToken        string `json:"adminToken"`
	UserToken         string `json:"userToken,omitempty"`
	CNI               string `json:"cni" default:"calico"`
	KubernetesVersion string `json:"kubernetesVersion"`
	NodeRoles         string `json:"nodeRoles,omitempty"`
	DefaultNamespace  string `default:"fleet-default"`
	Insecure          *bool  `json:"insecure" default:"true"`
	CAFile            string `json:"caFile" default:""`
	CACerts           string `json:"caCerts" default:""`
}
