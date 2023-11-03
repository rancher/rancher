package nodetemplates

// The json/yaml config key for the Amazon node template config
const AmazonEC2NodeTemplateConfigurationFileKey = "amazonec2Config"

// AmazonNodeTemplateConfig is configuration need to create a Amazon node template
type AmazonEC2NodeTemplateConfig struct {
	AccessKey               string   `json:"accessKey" yaml:"accessKey"`
	AMI                     string   `json:"ami" yaml:"ami"`
	BlockDurationMinutes    string   `json:"blockDurationMinutes" yaml:"blockDurationMinutes"`
	DeviceName              string   `json:"deviceName" yaml:"deviceName"`
	EncryptEBSVolume        bool     `json:"encryptEbsVolume" yaml:"encryptEbsVolume"`
	Endpoint                string   `json:"endpoint" yaml:"endpoint"`
	HTTPEndpoint            string   `json:"httpEndpoint" yaml:"httpEndpoint"`
	HTTPTokens              string   `json:"httpTokens" yaml:"httpTokens"`
	IAMInstanceProfile      string   `json:"iamInstanceProfile" yaml:"iamInstanceProfile"`
	InsecureTransport       bool     `json:"insecureTransport" yaml:"insecureTransport"`
	InstanceType            string   `json:"instanceType" yaml:"instanceType"`
	KeyPairName             string   `json:"keyPairName" yaml:"keyPairName"`
	KMSKey                  string   `json:"kmsKey" yaml:"kmsKey"`
	Monitoring              bool     `json:"monitoring" yaml:"monitoring"`
	PrivateAddressOnly      bool     `json:"privateAddressOnly" yaml:"privateAddressOnly"`
	Region                  string   `json:"region" yaml:"region"`
	RequestSpotInstance     bool     `json:"requestSpotInstance" yaml:"requestSpotInstance"`
	Retries                 string   `json:"retries" yaml:"retries"`
	RootSize                string   `json:"rootSize" yaml:"rootSize"`
	SecretKey               string   `json:"secretKey" yaml:"secretKey"`
	SecurityGroup           []string `json:"securityGroup" yaml:"securityGroup"`
	SecurityGroupReadonly   bool     `json:"securityGroupReadonly" yaml:"securityGroupReadonly"`
	SessionToken            string   `json:"sessionToken" yaml:"sessionToken"`
	SpotPrice               string   `json:"spotPrice" yaml:"spotPrice"`
	SSHKeyContexts          string   `json:"sshKeyContexts" yaml:"sshKeyContexts"`
	SSHUser                 string   `json:"sshUser" yaml:"sshUser"`
	SubnetID                string   `json:"subnetId" yaml:"subnetId"`
	Tags                    string   `json:"tags" yaml:"tags"`
	Type                    string   `json:"type" yaml:"type"`
	UsePrivateAddress       bool     `json:"usePrivateAddress" yaml:"usePrivateAddress"`
	UseEbsOptimizedInstance bool     `json:"useEbsOptimizedInstance" yaml:"useEbsOptimizedInstance"`
	VolumeType              string   `json:"volumeType" yaml:"volumeType"`
	VPCId                   string   `json:"vpcId" yaml:"vpcId"`
	Zone                    string   `json:"zone" yaml:"zone"`
}
