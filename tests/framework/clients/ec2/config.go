package ec2

const ConfigurationFileKey = "awsEC2Config"

type AWSEC2Config struct {
	Region             string   `json:"region" yaml:"region"`
	InstanceType       string   `json:"instanceType" yaml:"instanceType"`
	AWSRegionAZ        string   `json:"awsRegionAZ" yaml:"awsRegionAZ"`
	AWSAMI             string   `json:"awsAMI" yaml:"awsAMI"`
	AWSSecurityGroups  []string `json:"awsSecurityGroups" yaml:"awsSecurityGroups"`
	AWSAccessKeyID     string   `json:"awsAccessKeyID" yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string   `json:"awsSecretAccessKey" yaml:"awsSecretAccessKey"`
	AWSSSHKeyName      string   `json:"awsSSHKeyName" yaml:"awsSSHKeyName"`
	AWSCICDInstanceTag string   `json:"awsCICDInstanceTag" yaml:"awsCICDInstanceTag"`
	AWSIAMProfile      string   `json:"awsIAMProfile" yaml:"awsIAMProfile"`
	AWSUser            string   `json:"awsUser" yaml:"awsUser"`
	VolumeSize         int      `json:"volumeSize" yaml:"volumeSize"`
}
