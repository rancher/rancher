package nodetemplates

// The json/yaml config key for the Linode node template config
const LinodeNodeTemplateConfigurationFileKey = "linodeConfig"

// LinodeNodeTemplateConfig is configuration need to create a Linode node template
type LinodeNodeTemplateConfig struct {
	AuthorizedUsers string `json:"authorizedUsers" yaml:"authorizedUsers"`
	CreatePrivateIP bool   `json:"createPrivateIP" yaml:"createPrivateIP"`
	DockerPort      string `json:"dockerPort" yaml:"dockerPort"`
	Image           string `json:"image" yaml:"image"`
	InstanceType    string `json:"instanceType" yaml:"instanceType"`
	Label           string `json:"label" yaml:"label"`
	Region          string `json:"region" yaml:"region"`
	RootPass        string `json:"rootPass" yaml:"rootPass"`
	SSHPort         string `json:"sshPort" yaml:"sshPort"`
	SSHUser         string `json:"sshUser" yaml:"sshUser"`
	Stackscript     string `json:"stackscript" yaml:"stackscript"`
	StackscriptData string `json:"stackscriptData" yaml:"stackscriptData"`
	SwapSize        string `json:"swapSize" yaml:"swapSize"`
	Tags            string `json:"tags" yaml:"tags"`
	Type            string `json:"type" yaml:"type"`
	UAPrefix        string `json:"uaPrefix" yaml:"uaPrefix"`
}
