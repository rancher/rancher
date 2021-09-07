package config

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/creasty/defaults"
)

var (
	instance   *configuration = nil
	configPath                = os.Getenv("CONFIG_PATH")
)

type Configuration interface {
	// Rancher server
	SetCattleTestURL(url string)
	GetCattleTestURL() string
	SetAdminToken(token string)
	GetAdminToken() string
	SetUserToken(token string)
	GetUserToken() string
	SetRancherCleanup(cleanup bool)
	GetRancherCleanup() bool
	SetCNI(cni string)
	GetCNI() string
	SetKubernetesVersion(kubernetesVersion string)
	GetKubernetesVersion() string
	SetNodeRoles(nodeRoles string)
	GetNodeRoles() string
	SetDefaultNamespace(namespace string)
	GetDefaultNamespace() string
	SetInsecure(insecure bool)
	GetInsecure() bool
	SetCAFile(caFile string)
	GetCAFile() string
	SetCACerts(caCerts string)
	GetCACerts() string
	//Digital Ocean
	SetDOAccessKey(accessKey string)
	GetDOAccessKey() string
	SetDOImage(image string)
	GetDOImage() string
	SetDORegion(doRegion string)
	GetDORegion() string
	SetDOSize(size string)
	GetDOSize() string
	//AWS
	SetAWSInstanceType(instanceType string)
	GetAWSInstanceType() string
	SetAWSRegion(region string)
	GetAWSRegion() string
	SetAWSRegionAZ(regionAZ string)
	GetAWSRegionAZ() string
	SetAWSAMI(ami string)
	GetAWSAMI() string
	SetAWSSecurityGroup(sg string)
	GetAWSSecurityGroup() string
	SetAWSAccessKeyID(keyID string)
	GetAWSAccessKeyID() string
	SetAWSSecretAccessKey(secretKey string)
	GetAWSSecretAccessKey() string
	SetAWSSSHKeyName(sshKeyName string)
	GetAWSSSHKeyName() string
	SetAWSCICDInstanceTag(instanceTag string)
	GetAWSCICDInstanceTag() string
	SetAWSIAMProfile(profile string)
	GetAWSIAMProfile() string
	SetAWSUser(user string)
	GetAWSUser() string
	SetAWSVolumeSize(volumeSize int64)
	GetAWSVolumeSize() int64
	//OS specific
	SetSSHPath(sshPath string)
	GetSSHPath() string
}

type configuration struct {
	//Rancher server
	CattleTestURL     string `json:"CATTLE_TEST_URL"`
	AdminToken        string `json:"ADMIN_TOKEN"`
	UserToken         string `json:"USER_TOKEN,omitempty"`
	RancherCleanup    *bool  `json:"RANCHER_CLEANUP" default:"true"`
	CNI               string `json:"CNI" default:"calico"`
	KubernetesVersion string `json:"KUBERNETES_VERSION" default:"v1.21.5+rke2r2"`
	NodeRoles         string `json:"NODE_ROLES,omitempty"`
	DefaultNamespace  string `default:"fleet-default"`
	Insecure          *bool  `json:"INSECURE" default:"true"`
	CAFile            string `json:"CAFILE" default:""`
	CACerts           string `json:"CACERTS" default:""`
	//Digital Ocean
	DOAccessKey string `json:"DO_ACCESSKEY"`
	DOImage     string `json:"DO_IMAGE" default:"ubuntu-20-04-x64"`
	DORegion    string `json:"DO_REGION" default:"nyc3"`
	DOSize      string `json:"DOSize" default:"s-2vcpu-4gb"`
	//AWS
	AWSInstanceType    string `json:"AWS_INSTANCE_TYPE" default:"t3a.medium"`
	AWSRegion          string `json:"AWS_REGION" default:"us-east-2"`
	AWSRegionAZ        string `json:"AWS_REGION_AZ" default:""`
	AWSAMI             string `json:"AWS_AMI" default:"ami-0d5d9d301c853a04a"`
	AWSSecurityGroup   string `json:"AWS_SECURITY_GROUPS" default:"sg-0e753fd5550206e55"`
	AWSAccessKeyID     string `json:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `json:"AWS_SECRET_ACCESS_KEY"`
	AWSSSHKeyName      string `json:"AWS_SSH_KEY_NAME" default:"jenkins-rke-validation.pem"`
	AWSCICDInstanceTag string `json:"AWS_CICD_INSTANCE_TAG" default:"rancher-validation"`
	AWSIAMProfile      string `json:"AWS_IAM_PROFILE" default:""`
	AWSUser            string `json:"AWS_USER" default:"ubuntu"`
	AWSVolumeSize      int64  `json:"AWS_VOLUME_SIZE" default:"50"`
	//OS specific
	SSHPath string `json:"SSH_PATH,omitempty"`
}

func (c *configuration) SetCattleTestURL(url string) {
	c.CattleTestURL = url
}

func (c *configuration) GetCattleTestURL() string {
	return c.CattleTestURL
}

func (c *configuration) SetAdminToken(token string) {
	c.AdminToken = token
}

func (c *configuration) GetAdminToken() string {
	return c.AdminToken
}

func (c *configuration) SetUserToken(token string) {
	c.UserToken = token
}

func (c *configuration) GetUserToken() string {
	return c.UserToken
}

func (c *configuration) SetRancherCleanup(cleanup bool) {
	c.RancherCleanup = &cleanup
}

func (c *configuration) GetRancherCleanup() bool {
	return *c.RancherCleanup
}

func (c *configuration) SetCNI(cleanup string) {
	c.CNI = cleanup
}

func (c *configuration) GetCNI() string {
	return c.CNI
}

func (c *configuration) SetKubernetesVersion(kubernetesVersion string) {
	c.KubernetesVersion = kubernetesVersion
}

func (c *configuration) GetKubernetesVersion() string {
	return c.KubernetesVersion
}

func (c *configuration) SetNodeRoles(nodeRoles string) {
	c.NodeRoles = nodeRoles
}

func (c *configuration) GetNodeRoles() string {
	return c.NodeRoles
}

func (c *configuration) SetDefaultNamespace(namespace string) {
	c.DefaultNamespace = namespace
}

func (c *configuration) GetDefaultNamespace() string {
	return c.DefaultNamespace
}

func (c *configuration) SetInsecure(insecure bool) {
	c.Insecure = &insecure
}

func (c *configuration) GetInsecure() bool {
	return *c.Insecure
}

func (c *configuration) SetCAFile(caFile string) {
	c.CAFile = caFile
}

func (c *configuration) GetCAFile() string {
	return c.CAFile
}

func (c *configuration) SetCACerts(caCerts string) {
	c.CACerts = caCerts
}

func (c *configuration) GetCACerts() string {
	return c.CACerts
}

func (c *configuration) SetDOAccessKey(accessKey string) {
	c.DOAccessKey = accessKey
}

func (c *configuration) GetDOAccessKey() string {
	return c.DOAccessKey
}

func (c *configuration) SetDOImage(image string) {
	c.DOImage = image
}

func (c *configuration) GetDOImage() string {
	return c.DOImage
}

func (c *configuration) SetDORegion(doRegion string) {
	c.DORegion = doRegion
}

func (c *configuration) GetDORegion() string {
	return c.DORegion
}

func (c *configuration) SetDOSize(size string) {
	c.DOSize = size
}

func (c *configuration) GetDOSize() string {
	return c.DOSize
}

func (c *configuration) SetAWSInstanceType(instanceType string) {
	c.AWSInstanceType = instanceType
}

func (c *configuration) GetAWSInstanceType() string {
	return c.AWSInstanceType
}

func (c *configuration) SetAWSRegion(region string) {
	c.AWSRegion = region
}

func (c *configuration) GetAWSRegion() string {
	return c.AWSRegion
}

func (c *configuration) SetAWSRegionAZ(regionAZ string) {
	c.AWSRegionAZ = regionAZ
}

func (c *configuration) GetAWSRegionAZ() string {
	return c.AWSRegionAZ
}

func (c *configuration) SetAWSAMI(ami string) {
	c.AWSAMI = ami
}

func (c *configuration) GetAWSAMI() string {
	return c.AWSAMI
}

func (c *configuration) SetAWSSecurityGroup(sg string) {
	c.AWSSecurityGroup = sg
}

func (c *configuration) GetAWSSecurityGroup() string {
	return c.AWSSecurityGroup
}

func (c *configuration) SetAWSAccessKeyID(keyID string) {
	c.AWSAccessKeyID = keyID
}

func (c *configuration) GetAWSAccessKeyID() string {
	return c.AWSAccessKeyID
}

func (c *configuration) SetAWSSecretAccessKey(secretKey string) {
	c.AWSSecretAccessKey = secretKey
}

func (c *configuration) GetAWSSecretAccessKey() string {
	return c.AWSSecretAccessKey
}

func (c *configuration) SetAWSSSHKeyName(sshKeyName string) {
	c.AWSSSHKeyName = sshKeyName
}

func (c *configuration) GetAWSSSHKeyName() string {
	return c.AWSSSHKeyName
}

func (c *configuration) SetAWSCICDInstanceTag(instanceTag string) {
	c.AWSCICDInstanceTag = instanceTag
}

func (c *configuration) GetAWSCICDInstanceTag() string {
	return c.AWSCICDInstanceTag
}

func (c *configuration) SetAWSIAMProfile(profile string) {
	c.AWSIAMProfile = profile
}

func (c *configuration) GetAWSIAMProfile() string {
	return c.AWSIAMProfile
}

func (c *configuration) SetAWSUser(user string) {
	c.AWSUser = user
}

func (c *configuration) GetAWSUser() string {
	return c.AWSUser
}

func (c *configuration) SetAWSVolumeSize(volumeSize int64) {
	c.AWSVolumeSize = volumeSize
}

func (c *configuration) GetAWSVolumeSize() int64 {
	return c.AWSVolumeSize
}

func (c *configuration) SetSSHPath(sshPath string) {
	c.SSHPath = sshPath
}

func (c *configuration) GetSSHPath() string {
	return c.SSHPath
}

func init() {
	var conf configuration
	if configPath != "" {
		configContents, err := ioutil.ReadFile(configPath)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(configContents, &conf)
		if err != nil {
			panic(err)
		}
	}

	if err := defaults.Set(&conf); err != nil {
		panic(err)
	}
	instance = &conf

}

func GetInstance() Configuration {
	return instance
}
