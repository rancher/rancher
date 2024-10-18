package observability


type StackStateConfigs struct {
	ServiceToken  string `json:"serviceToken" yaml:"serviceToken"`
	Url           string `json:"url" yaml:"url"`
	ClusterApiKey string `json:"clusterApiKey" yaml:"clusterApiKey"`
}