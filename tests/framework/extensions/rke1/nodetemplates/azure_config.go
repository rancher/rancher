package nodetemplates

// The json/yaml config key for the Azure node template config
const AzureNodeTemplateConfigurationFileKey = "azureNodeTemplate"

// AzureNodeTemplateConfig is configuration need to create a Azure node template
type AzureNodeTemplateConfig struct {
	AvailabilitySet   string   `json:"availabilitySet" yaml:"availabilitySet"`
	ClientId          string   `json:"clientId" yaml:"clientId"`
	ClientSecret      string   `json:"clientSecret" yaml:"clientSecret"`
	CustomData        string   `json:"customData" yaml:"customData"`
	DiskSize          string   `json:"diskSize" yaml:"diskSize"`
	DNS               string   `json:"dns" yaml:"dns"`
	DockerPort        string   `json:"dockerPort" yaml:"dockerPort"`
	Environment       string   `json:"environment" yaml:"environment"`
	FaultDomainCount  string   `json:"faultDomainCount" yaml:"faultDomainCount"`
	Image             string   `json:"image" yaml:"image"`
	Location          string   `json:"location" yaml:"location"`
	ManagedDisks      bool     `json:"managedDisks" yaml:"managedDisks"`
	NoPublicIp        bool     `json:"noPublicIp" yaml:"noPublicIp"`
	OpenPort          []string `json:"openPort" yaml:"openPort"`
	Plan              string   `json:"plan" yaml:"plan"`
	PrivateIPAddress  string   `json:"privateIpAddress" yaml:"privateIpAddress"`
	ResourceGroup     string   `json:"resourceGroup" yaml:"resourceGroup"`
	Size              string   `json:"size" yaml:"size"`
	SSHUser           string   `json:"sshUser" yaml:"sshUser"`
	StaticPublicIp    bool     `json:"staticPublicIp" yaml:"staticPublicIp"`
	StorageType       string   `json:"storageType" yaml:"storageType"`
	Subnet            string   `json:"subnet" yaml:"subnet"`
	SubnetPrefix      string   `json:"subnetPrefix" yaml:"subnetPrefix"`
	SubscriptionId    string   `json:"subscriptionId" yaml:"subscriptionId"`
	TenantId          string   `json:"tenantId" yaml:"tenantId"`
	Type              string   `json:"type" yaml:"type"`
	UpdateDomainCount string   `json:"updateDomainCount" yaml:"updateDomainCount"`
	UsePrivatIp       bool     `json:"usePrivateIp" yaml:"usePrivateIp"`
	VNET              string   `json:"vnet" yaml:"vnet"`
}
