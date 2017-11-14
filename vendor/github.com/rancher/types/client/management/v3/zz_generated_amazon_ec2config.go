package client

const (
	AmazonEC2ConfigType                         = "amazonEC2Config"
	AmazonEC2ConfigFieldAccessKey               = "accessKey"
	AmazonEC2ConfigFieldAmi                     = "ami"
	AmazonEC2ConfigFieldBlockDurationMinutes    = "blockDurationMinutes"
	AmazonEC2ConfigFieldDeviceName              = "deviceName"
	AmazonEC2ConfigFieldEndpoint                = "endpoint"
	AmazonEC2ConfigFieldIamInstanceProfile      = "iamInstanceProfile"
	AmazonEC2ConfigFieldInsecureTransport       = "insecureTransport"
	AmazonEC2ConfigFieldInstanceType            = "instanceType"
	AmazonEC2ConfigFieldKeypairName             = "keypairName"
	AmazonEC2ConfigFieldMonitoring              = "monitoring"
	AmazonEC2ConfigFieldOpenPort                = "openPort"
	AmazonEC2ConfigFieldPrivateAddressOnly      = "privateAddressOnly"
	AmazonEC2ConfigFieldRegion                  = "region"
	AmazonEC2ConfigFieldRequestSpotInstance     = "requestSpotInstance"
	AmazonEC2ConfigFieldRetries                 = "retries"
	AmazonEC2ConfigFieldRootSize                = "rootSize"
	AmazonEC2ConfigFieldSSHKeypath              = "sshKeypath"
	AmazonEC2ConfigFieldSSHUser                 = "sshUser"
	AmazonEC2ConfigFieldSecretKey               = "secretKey"
	AmazonEC2ConfigFieldSecurityGroup           = "securityGroup"
	AmazonEC2ConfigFieldSessionToken            = "sessionToken"
	AmazonEC2ConfigFieldSpotPrice               = "spotPrice"
	AmazonEC2ConfigFieldSubnetID                = "subnetId"
	AmazonEC2ConfigFieldTags                    = "tags"
	AmazonEC2ConfigFieldUseEbsOptimizedInstance = "useEbsOptimizedInstance"
	AmazonEC2ConfigFieldUsePrivateAddress       = "usePrivateAddress"
	AmazonEC2ConfigFieldUserdata                = "userdata"
	AmazonEC2ConfigFieldVolumeType              = "volumeType"
	AmazonEC2ConfigFieldVpcID                   = "vpcId"
	AmazonEC2ConfigFieldZone                    = "zone"
)

type AmazonEC2Config struct {
	AccessKey               string   `json:"accessKey,omitempty"`
	Ami                     string   `json:"ami,omitempty"`
	BlockDurationMinutes    string   `json:"blockDurationMinutes,omitempty"`
	DeviceName              string   `json:"deviceName,omitempty"`
	Endpoint                string   `json:"endpoint,omitempty"`
	IamInstanceProfile      string   `json:"iamInstanceProfile,omitempty"`
	InsecureTransport       *bool    `json:"insecureTransport,omitempty"`
	InstanceType            string   `json:"instanceType,omitempty"`
	KeypairName             string   `json:"keypairName,omitempty"`
	Monitoring              *bool    `json:"monitoring,omitempty"`
	OpenPort                []string `json:"openPort,omitempty"`
	PrivateAddressOnly      *bool    `json:"privateAddressOnly,omitempty"`
	Region                  string   `json:"region,omitempty"`
	RequestSpotInstance     *bool    `json:"requestSpotInstance,omitempty"`
	Retries                 string   `json:"retries,omitempty"`
	RootSize                string   `json:"rootSize,omitempty"`
	SSHKeypath              string   `json:"sshKeypath,omitempty"`
	SSHUser                 string   `json:"sshUser,omitempty"`
	SecretKey               string   `json:"secretKey,omitempty"`
	SecurityGroup           []string `json:"securityGroup,omitempty"`
	SessionToken            string   `json:"sessionToken,omitempty"`
	SpotPrice               string   `json:"spotPrice,omitempty"`
	SubnetID                string   `json:"subnetId,omitempty"`
	Tags                    string   `json:"tags,omitempty"`
	UseEbsOptimizedInstance *bool    `json:"useEbsOptimizedInstance,omitempty"`
	UsePrivateAddress       *bool    `json:"usePrivateAddress,omitempty"`
	Userdata                string   `json:"userdata,omitempty"`
	VolumeType              string   `json:"volumeType,omitempty"`
	VpcID                   string   `json:"vpcId,omitempty"`
	Zone                    string   `json:"zone,omitempty"`
}
