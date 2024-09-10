package nodetemplates

// The json/yaml config key for the Azure node template config
const AzureNodeTemplateConfigurationFileKey = "azureConfig"

// AzureNodeTemplateConfig is configuration need to create a Azure node template
type AzureNodeTemplateConfig struct {
	AvailabilitySet   string   `json:"availabilitySet" yaml:"availabilitySet"`
	CustomData        string   `json:"customData" yaml:"customData"`
	DiskSize          string   `json:"diskSize" yaml:"diskSize"`
	DNS               string   `json:"dns" yaml:"dns"`
	DockerPort        string   `json:"dockerPort" yaml:"dockerPort"`
	Environment       string   `json:"environment" yaml:"environment"`
	FaultDomainCount  string   `json:"faultDomainCount" yaml:"faultDomainCount"`
	Image             string   `json:"image" yaml:"image"`
	Location          string   `json:"location" yaml:"location"`
	ManagedDisks      bool     `json:"managedDisks" yaml:"managedDisks"`
	NoPublicIP        bool     `json:"noPublicIp" yaml:"noPublicIp"`
	OpenPort          []string `json:"openPort" yaml:"openPort"`
	NSG               string   `json:"nsg" yaml:"nsg"`
	Plan              string   `json:"plan" yaml:"plan"`
	PrivateIPAddress  string   `json:"privateIpAddress" yaml:"privateIpAddress"`
	ResourceGroup     string   `json:"resourceGroup" yaml:"resourceGroup"`
	Size              string   `json:"size" yaml:"size"`
	SSHUser           string   `json:"sshUser" yaml:"sshUser"`
	StaticPublicIP    bool     `json:"staticPublicIp" yaml:"staticPublicIp"`
	StorageType       string   `json:"storageType" yaml:"storageType"`
	Subnet            string   `json:"subnet" yaml:"subnet"`
	SubnetPrefix      string   `json:"subnetPrefix" yaml:"subnetPrefix"`
	Type              string   `json:"type" yaml:"type"`
	UpdateDomainCount string   `json:"updateDomainCount" yaml:"updateDomainCount"`
	UsePrivateIP      bool     `json:"usePrivateIp" yaml:"usePrivateIp"`
	VNET              string   `json:"vnet" yaml:"vnet"`
}
