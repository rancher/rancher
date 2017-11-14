package client

const (
	AzureConfigType                  = "azureConfig"
	AzureConfigFieldAvailabilitySet  = "availabilitySet"
	AzureConfigFieldClientID         = "clientId"
	AzureConfigFieldClientSecret     = "clientSecret"
	AzureConfigFieldCustomData       = "customData"
	AzureConfigFieldDNS              = "dns"
	AzureConfigFieldDockerPort       = "dockerPort"
	AzureConfigFieldEnvironment      = "environment"
	AzureConfigFieldImage            = "image"
	AzureConfigFieldLocation         = "location"
	AzureConfigFieldNoPublicIP       = "noPublicIp"
	AzureConfigFieldOpenPort         = "openPort"
	AzureConfigFieldPrivateIPAddress = "privateIpAddress"
	AzureConfigFieldResourceGroup    = "resourceGroup"
	AzureConfigFieldSSHUser          = "sshUser"
	AzureConfigFieldSize             = "size"
	AzureConfigFieldStaticPublicIP   = "staticPublicIp"
	AzureConfigFieldStorageType      = "storageType"
	AzureConfigFieldSubnet           = "subnet"
	AzureConfigFieldSubnetPrefix     = "subnetPrefix"
	AzureConfigFieldSubscriptionID   = "subscriptionId"
	AzureConfigFieldUsePrivateIP     = "usePrivateIp"
	AzureConfigFieldVnet             = "vnet"
)

type AzureConfig struct {
	AvailabilitySet  string   `json:"availabilitySet,omitempty"`
	ClientID         string   `json:"clientId,omitempty"`
	ClientSecret     string   `json:"clientSecret,omitempty"`
	CustomData       string   `json:"customData,omitempty"`
	DNS              string   `json:"dns,omitempty"`
	DockerPort       string   `json:"dockerPort,omitempty"`
	Environment      string   `json:"environment,omitempty"`
	Image            string   `json:"image,omitempty"`
	Location         string   `json:"location,omitempty"`
	NoPublicIP       *bool    `json:"noPublicIp,omitempty"`
	OpenPort         []string `json:"openPort,omitempty"`
	PrivateIPAddress string   `json:"privateIpAddress,omitempty"`
	ResourceGroup    string   `json:"resourceGroup,omitempty"`
	SSHUser          string   `json:"sshUser,omitempty"`
	Size             string   `json:"size,omitempty"`
	StaticPublicIP   *bool    `json:"staticPublicIp,omitempty"`
	StorageType      string   `json:"storageType,omitempty"`
	Subnet           string   `json:"subnet,omitempty"`
	SubnetPrefix     string   `json:"subnetPrefix,omitempty"`
	SubscriptionID   string   `json:"subscriptionId,omitempty"`
	UsePrivateIP     *bool    `json:"usePrivateIp,omitempty"`
	Vnet             string   `json:"vnet,omitempty"`
}
