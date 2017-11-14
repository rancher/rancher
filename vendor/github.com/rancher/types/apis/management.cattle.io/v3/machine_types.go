package v3

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MachineTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec MachineTemplateSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status MachineTemplateStatus `json:"status"`
}

type MachineTemplateStatus struct {
	Conditions []MachineTemplateCondition `json:"conditions"`
}

type MachineTemplateCondition struct {
	// Type of cluster condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
}

type MachineTemplateSpec struct {
	DisplayName  string            `json:"displayName"`
	Description  string            `json:"description"`
	FlavorPrefix string            `json:"flavorPrefix"`
	Driver       string            `json:"driver"`
	SecretValues map[string]string `json:"secretValues"`
	SecretName   string            `json:"secretName"`
	PublicValues map[string]string `json:"publicValues"`
}

type Machine struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec MachineSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status MachineStatus `json:"status"`
}

type MachineStatus struct {
	Conditions      []MachineCondition `json:"conditions"`
	NodeStatus      v1.NodeStatus      `json:"nodeStatus"`
	NodeName        string             `json:"nodeName"`
	Requested       v1.ResourceList    `json:"requested,omitempty"`
	Limits          v1.ResourceList    `json:"limits,omitempty"`
	Provisioned     bool               `json:"provisioned,omitempty"`
	SSHPrivateKey   string             `json:"sshPrivateKey,omitempty"`
	ExtractedConfig string             `json:"extractedConfig,omitempty"`
}

type MachineCondition struct {
	// Type of cluster condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
}

type MachineSpec struct {
	NodeSpec            v1.NodeSpec `json:"nodeSpec"`
	ClusterName         string      `json:"clusterName" norman:"type=reference[cluster]"`
	MachineTemplateName string      `json:"machineTemplateName" norman:"type=reference[machineTemplate]"`
	Description         string      `json:"description"`
	Driver              string      `json:"driver"`

	MachineCommonParams `json:",inline"`
	AmazonEC2Config     AmazonEC2Config    `json:"amazonEc2Config"`
	AzureConfig         AzureConfig        `json:"azureConfig"`
	DigitalOceanConfig  DigitalOceanConfig `json:"digitalOceanConfig"`
}

type AmazonEC2Config struct {
	AccessKey string `json:"accessKey,omitempty"`

	Ami string `json:"ami,omitempty"`

	BlockDurationMinutes string `json:"blockDurationMinutes,omitempty"`

	DeviceName string `json:"deviceName,omitempty"`

	Endpoint string `json:"endpoint,omitempty"`

	IamInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	InsecureTransport bool `json:"insecureTransport,omitempty"`

	InstanceType string `json:"instanceType,omitempty"`

	KeypairName string `json:"keypairName,omitempty"`

	Monitoring bool `json:"monitoring,omitempty"`

	OpenPort []string `json:"openPort,omitempty"`

	PrivateAddressOnly bool `json:"privateAddressOnly,omitempty"`

	Region string `json:"region,omitempty"`

	RequestSpotInstance bool `json:"requestSpotInstance,omitempty"`

	Retries string `json:"retries,omitempty"`

	RootSize string `json:"rootSize,omitempty"`

	SecretKey string `json:"secretKey,omitempty"`

	SecurityGroup []string `json:"securityGroup,omitempty"`

	SessionToken string `json:"sessionToken,omitempty"`

	SpotPrice string `json:"spotPrice,omitempty"`

	SSHKeypath string `json:"sshKeypath,omitempty"`

	SSHUser string `json:"sshUser,omitempty"`

	SubnetID string `json:"subnetId,omitempty"`

	Tags string `json:"tags,omitempty"`

	UseEbsOptimizedInstance bool `json:"useEbsOptimizedInstance,omitempty"`

	UsePrivateAddress bool `json:"usePrivateAddress,omitempty"`

	Userdata string `json:"userdata,omitempty"`

	VolumeType string `json:"volumeType,omitempty"`

	VpcID string `json:"vpcId,omitempty"`

	Zone string `json:"zone,omitempty"`
}

type AzureConfig struct {
	AvailabilitySet string `json:"availabilitySet,omitempty"`

	ClientID string `json:"clientId,omitempty"`

	ClientSecret string `json:"clientSecret,omitempty"`

	CustomData string `json:"customData,omitempty"`

	DNS string `json:"dns,omitempty"`

	DockerPort string `json:"dockerPort,omitempty"`

	Environment string `json:"environment,omitempty"`

	Image string `json:"image,omitempty"`

	Location string `json:"location,omitempty"`

	NoPublicIP bool `json:"noPublicIp,omitempty"`

	OpenPort []string `json:"openPort,omitempty"`

	PrivateIPAddress string `json:"privateIpAddress,omitempty"`

	ResourceGroup string `json:"resourceGroup,omitempty"`

	Size string `json:"size,omitempty"`

	SSHUser string `json:"sshUser,omitempty"`

	StaticPublicIP bool `json:"staticPublicIp,omitempty"`

	StorageType string `json:"storageType,omitempty"`

	Subnet string `json:"subnet,omitempty"`

	SubnetPrefix string `json:"subnetPrefix,omitempty"`

	SubscriptionID string `json:"subscriptionId,omitempty"`

	UsePrivateIP bool `json:"usePrivateIp,omitempty"`

	Vnet string `json:"vnet,omitempty"`
}

type DigitalOceanConfig struct {
	AccessToken string `json:"accessToken,omitempty"`

	Backups bool `json:"backups,omitempty"`

	Image string `json:"image,omitempty"`

	Ipv6 bool `json:"ipv6,omitempty"`

	PrivateNetworking bool `json:"privateNetworking,omitempty"`

	Region string `json:"region,omitempty"`

	Size string `json:"size,omitempty"`

	SSHKeyFingerprint string `json:"sshKeyFingerprint,omitempty"`

	SSHKeyPath string `json:"sshKeyPath,omitempty"`

	SSHPort string `json:"sshPort,omitempty"`

	SSHUser string `json:"sshUser,omitempty"`

	Userdata string `json:"userdata,omitempty"`
}

type MachineCommonParams struct {
	AuthCertificateAuthority string            `json:"authCertificateAuthority"`
	AuthKey                  string            `json:"authKey"`
	EngineInstallURL         string            `json:"engineInstallURL"`
	DockerVersion            string            `json:"dockerVersion"`
	EngineOpt                map[string]string `json:"engineOpt"`
	EngineInsecureRegistry   []string          `json:"engineInsecureRegistry"`
	EngineRegistryMirror     []string          `json:"engineRegistryMirror"`
	EngineLabel              map[string]string `json:"engineLabel"`
	EngineStorageDriver      string            `json:"engineStorageDriver"`
	EngineEnv                map[string]string `json:"engineEnv"`
}

type MachineDriver struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec MachineDriverSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status MachineDriverStatus `json:"status"`
}

type MachineDriverStatus struct {
	Conditions []MachineDriverCondition `json:"conditions"`
}

type MachineDriverCondition struct {
	// Type of cluster condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
}

type MachineDriverSpec struct {
	DisplayName      string `json:"displayName"`
	Description      string `json:"description"`
	URL              string `json:"url"`
	ExternalID       string `json:"externalId"`
	Builtin          bool   `json:"builtin"`
	DefaultActive    bool   `json:"defaultActive"`
	ActivateOnCreate bool   `json:"activateOnCreate"`
	Checksum         string `json:"checksum"`
	UIURL            string `json:"uiUrl"`
}
