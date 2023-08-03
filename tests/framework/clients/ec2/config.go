package ec2

// The json/yaml config key for the AWSEC2onfig
const ConfigurationFileKey = "awsEC2Configs"

// AWSEC2Configs is the AWS authentication configuration for accessing and launching ec2 instances
type AWSEC2Configs struct {
	AWSEC2Config       []AWSEC2Config `json:"awsEC2Config" yaml:"awsEC2Config"`
	AWSAccessKeyID     string         `json:"awsAccessKeyID" yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string         `json:"awsSecretAccessKey" yaml:"awsSecretAccessKey"`
	Region             string         `json:"region" yaml:"region"`
}

// AWSEC2Config is the instance-specific configuration needed to launch ec2 instances in AWS
type AWSEC2Config struct {
	InstanceType       string   `json:"instanceType" yaml:"instanceType"`
	AWSRegionAZ        string   `json:"awsRegionAZ" yaml:"awsRegionAZ"`
	AWSAMI             string   `json:"awsAMI" yaml:"awsAMI"`
	AWSSecurityGroups  []string `json:"awsSecurityGroups" yaml:"awsSecurityGroups"`
	AWSSSHKeyName      string   `json:"awsSSHKeyName" yaml:"awsSSHKeyName"`
	AWSCICDInstanceTag string   `json:"awsCICDInstanceTag" yaml:"awsCICDInstanceTag"`
	AWSIAMProfile      string   `json:"awsIAMProfile" yaml:"awsIAMProfile"`
	AWSUser            string   `json:"awsUser" yaml:"awsUser"`
	VolumeSize         int      `json:"volumeSize" yaml:"volumeSize"`
	Roles              []string `json:"roles" yaml:"roles"`
}
