package observability

type StackStateConfig struct {
	ServiceToken   string `json:"serviceToken" yaml:"serviceToken"`
	Url            string `json:"url" yaml:"url"`
	ClusterApiKey  string `json:"clusterApiKey" yaml:"clusterApiKey"`
	UpgradeVersion string `json:"upgradeVersion" yaml:"upgradeVersion"`
}
