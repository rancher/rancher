package cloudcredentials

// The json/yaml config key for the azure cloud credential config
const VmwarevsphereCredentialConfigurationFileKey = "vmwarevsphereCredentials"

// VmwareVsphereCredentialConfig is configuration need to create an vsphere cloud credential
type VmwarevsphereCredentialConfig struct {
	Password    string `json:"password" yaml:"password"`
	Username    string `json:"username" yaml:"username"`
	Vcenter     string `json:"vcenter" yaml:"vcenter"`
	VcenterPort string `json:"vcenterPort" yaml:"vcenterPort"`
}
