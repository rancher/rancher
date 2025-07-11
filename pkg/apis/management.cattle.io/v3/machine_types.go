package v3

import (
	"time"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	rketypes "github.com/rancher/rke/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodeTemplate struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec NodeTemplateSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status NodeTemplateStatus `json:"status"`
}

type NodeTemplateStatus struct {
	Conditions []NodeTemplateCondition `json:"conditions"`
}

type NodeTemplateCondition struct {
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

type NodeTemplateSpec struct {
	DisplayName         string     `json:"displayName"`
	Description         string     `json:"description"`
	Driver              string     `json:"driver" norman:"nocreate,noupdate"`
	CloudCredentialName string     `json:"cloudCredentialName" norman:"type=reference[cloudCredential]"`
	NodeTaints          []v1.Taint `json:"nodeTaints,omitempty"`
	NodeCommonParams    `json:",inline"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Node struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec NodeSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status NodeStatus `json:"status"`
}

func (in *Node) ObjClusterName() string {
	return in.Namespace
}

type MetadataUpdate struct {
	Labels      MapDelta `json:"labels,omitempty"`
	Annotations MapDelta `json:"annotations,omitempty"`
}

type MapDelta struct {
	Add    map[string]string `json:"add,omitempty"`
	Delete map[string]bool   `json:"delete,omitempty"`
}

type NodeStatus struct {
	Conditions         []NodeCondition         `json:"conditions,omitempty"`
	InternalNodeStatus v1.NodeStatus           `json:"internalNodeStatus,omitempty"`
	NodeName           string                  `json:"nodeName,omitempty"`
	Requested          v1.ResourceList         `json:"requested,omitempty"`
	Limits             v1.ResourceList         `json:"limits,omitempty"`
	NodeTemplateSpec   *NodeTemplateSpec       `json:"nodeTemplateSpec,omitempty"`
	NodeConfig         *rketypes.RKEConfigNode `json:"rkeNode,omitempty"`
	NodeAnnotations    map[string]string       `json:"nodeAnnotations,omitempty"`
	NodeLabels         map[string]string       `json:"nodeLabels,omitempty"`
	NodeTaints         []v1.Taint              `json:"nodeTaints,omitempty"`
	DockerInfo         *DockerInfo             `json:"dockerInfo,omitempty"`
	NodePlan           *NodePlan               `json:"nodePlan,omitempty"`
	AppliedNodeVersion int                     `json:"appliedNodeVersion,omitempty"`
}

type DockerInfo struct {
	ID                 string
	Driver             string
	Debug              bool
	LoggingDriver      string
	CgroupDriver       string
	KernelVersion      string
	OperatingSystem    string
	OSType             string
	Architecture       string
	IndexServerAddress string
	InitBinary         string
	DockerRootDir      string
	SecurityOptions    []string
	HTTPProxy          string
	HTTPSProxy         string
	NoProxy            string
	Name               string
	Labels             []string
	ExperimentalBuild  bool
	ServerVersion      string
}

var (
	NodeConditionInitialized condition.Cond = "Initialized"
	NodeConditionProvisioned condition.Cond = "Provisioned"
	NodeConditionUpdated     condition.Cond = "Updated"
	NodeConditionRegistered  condition.Cond = "Registered"
	NodeConditionRemoved     condition.Cond = "Removed"
	NodeConditionConfigSaved condition.Cond = "Saved"
	NodeConditionReady       condition.Cond = "Ready"
	NodeConditionDrained     condition.Cond = "Drained"
	NodeConditionUpgraded    condition.Cond = "Upgraded"
)

type NodeCondition struct {
	// Type of cluster condition.
	Type condition.Cond `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

var (
	NodePoolConditionUpdated condition.Cond = "Updated"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodePool struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodePoolSpec   `json:"spec"`
	Status NodePoolStatus `json:"status"`
}

func (n *NodePool) ObjClusterName() string {
	return n.Spec.ObjClusterName()
}

type NodePoolSpec struct {
	Etcd             bool   `json:"etcd"`
	ControlPlane     bool   `json:"controlPlane"`
	Worker           bool   `json:"worker"`
	NodeTemplateName string `json:"nodeTemplateName,omitempty" norman:"type=reference[nodeTemplate],required,notnullable"`

	HostnamePrefix    string            `json:"hostnamePrefix" norman:"required,notnullable"`
	Quantity          int               `json:"quantity" norman:"required,default=1"`
	DrainBeforeDelete bool              `json:"drainBeforeDelete" norman:"default=false"`
	NodeLabels        map[string]string `json:"nodeLabels"`
	NodeAnnotations   map[string]string `json:"nodeAnnotations"`
	NodeTaints        []v1.Taint        `json:"nodeTaints,omitempty"`

	DisplayName string `json:"displayName"`
	ClusterName string `json:"clusterName,omitempty" norman:"type=reference[cluster],noupdate,required"`

	DeleteNotReadyAfterSecs time.Duration `json:"deleteNotReadyAfterSecs" norman:"default=0,max=31540000,min=0"`
}

func (n *NodePoolSpec) ObjClusterName() string {
	return n.ClusterName
}

type NodePoolStatus struct {
	Conditions []Condition `json:"conditions"`
}

type CustomConfig struct {
	// IP or FQDN that is fully resolvable and used for SSH communication
	Address string `yaml:"address" json:"address,omitempty"`
	// Optional - Internal address that will be used for components communication
	InternalAddress string `yaml:"internal_address" json:"internalAddress,omitempty"`
	// SSH user that will be used by RKE
	User string `yaml:"user" json:"user,omitempty"`
	// Optional - Docker socket on the node that will be used in tunneling
	DockerSocket string `yaml:"docker_socket" json:"dockerSocket,omitempty"`
	// SSH Private Key
	SSHKey string `yaml:"ssh_key" json:"sshKey,omitempty" norman:"type=password"`
	// SSH Certificate
	SSHCert string            `yaml:"ssh_cert" json:"sshCert,omitempty"`
	Label   map[string]string `yaml:"label" json:"label,omitempty"`
	Taints  []string          `yaml:"taints" json:"taints,omitempty"`
}

type NodeSpec struct {
	// Common fields.  They aren't in a shared struct because the annotations are different

	Etcd             bool   `json:"etcd" norman:"noupdate"`
	ControlPlane     bool   `json:"controlPlane" norman:"noupdate"`
	Worker           bool   `json:"worker" norman:"noupdate"`
	NodeTemplateName string `json:"nodeTemplateName,omitempty" norman:"type=reference[nodeTemplate],noupdate"`

	NodePoolName             string          `json:"nodePoolName" norman:"type=reference[nodePool],nocreate,noupdate"`
	CustomConfig             *CustomConfig   `json:"customConfig"`
	Imported                 bool            `json:"imported"`
	Description              string          `json:"description,omitempty"`
	DisplayName              string          `json:"displayName"`
	RequestedHostname        string          `json:"requestedHostname,omitempty" norman:"type=hostname,nullable,noupdate,required"`
	InternalNodeSpec         v1.NodeSpec     `json:"internalNodeSpec"`
	DesiredNodeTaints        []v1.Taint      `json:"desiredNodeTaints"`
	UpdateTaintsFromAPI      *bool           `json:"updateTaintsFromAPI,omitempty"`
	DesiredNodeUnschedulable string          `json:"desiredNodeUnschedulable,omitempty"`
	NodeDrainInput           *NodeDrainInput `json:"nodeDrainInput,omitempty"`
	MetadataUpdate           MetadataUpdate  `json:"metadataUpdate,omitempty"`
	ScaledownTime            string          `json:"scaledownTime,omitempty"`
}

type NodePlan struct {
	Plan    *rketypes.RKEConfigNodePlan `json:"plan,omitempty"`
	Version int                         `json:"version,omitempty"`
	// current default in rancher-agent is 2m (120s)
	AgentCheckInterval int `json:"agentCheckInterval,omitempty" norman:"min=1,max=1800,default=120"`
}

type NodeCommonParams struct {
	AuthCertificateAuthority string            `json:"authCertificateAuthority,omitempty"`
	AuthKey                  string            `json:"authKey,omitempty"`
	EngineInstallURL         string            `json:"engineInstallURL,omitempty"`
	DockerVersion            string            `json:"dockerVersion,omitempty"`
	EngineOpt                map[string]string `json:"engineOpt,omitempty"`
	StorageOpt               map[string]string `json:"storageOpt,omitempty"`
	LogOpt                   map[string]string `json:"logOpt,omitempty"`
	EngineInsecureRegistry   []string          `json:"engineInsecureRegistry,omitempty"`
	EngineRegistryMirror     []string          `json:"engineRegistryMirror,omitempty"`
	EngineLabel              map[string]string `json:"engineLabel,omitempty"`
	EngineStorageDriver      string            `json:"engineStorageDriver,omitempty"`
	EngineEnv                map[string]string `json:"engineEnv,omitempty"`
	UseInternalIPAddress     *bool             `json:"useInternalIpAddress,omitempty" norman:"default=true,noupdate"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Activated",type=string,JSONPath=".spec.active"
// +kubebuilder:printcolumn:name="Binary URL",type=string,JSONPath=".spec.url"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeDriver represents a Rancher node driver for a specific cloud provider used to provision cluster nodes.
type NodeDriver struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the Node Driver. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +kubebuilder:validation:XValidation:rule="!has(self.checksum) || (self.checksum.size() in [0, 32, 40, 64, 128])",message="Checksum must be an md5, sha1, sha256, or sha512 digest."
	Spec NodeDriverSpec `json:"spec"`
	// Most recent observed status of the Node Driver. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status NodeDriverStatus `json:"status,omitempty"`
}

type NodeDriverStatus struct {
	// Conditions is a representation of the current state of the driver,
	// this includes its installation status (Downloaded, Installed), as well
	// as its current state (Active, Inactive). Information related to
	// errors encountered while transitioning to one of these states will be
	// populated in the Message and Reason fields.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// AppliedURL is the url last used to download the driver.
	// +optional
	AppliedURL string `json:"appliedURL,omitempty"`
	// AppliedChecksum is the last known checksum of the driver. This is used
	// to determine when a Driver needs to be redownloaded from its URL.
	// +optional
	AppliedChecksum string `json:"appliedChecksum,omitempty"`
	// AppliedDockerMachineVersion specifies the last docker machine version
	// (a.k.a rancher-machine) which provides this driver. When this version
	// is incremented, Rancher will query the rancher-machine driver to
	// obtain its arguments and update the automatically generated schema
	// and associated machine config object. This field is only specified for
	// drivers bundled within Rancher via rancher-machine.
	// +optional
	AppliedDockerMachineVersion string `json:"appliedDockerMachineVersion,omitempty"`
}

var (
	NodeDriverConditionDownloaded condition.Cond = "Downloaded"
	NodeDriverConditionInstalled  condition.Cond = "Installed"
	NodeDriverConditionActive     condition.Cond = "Active"
	NodeDriverConditionInactive   condition.Cond = "Inactive"
)

type NodeDriverSpec struct {
	// DisplayName specifies the publicly visible name of the driver shown in the Rancher UI.
	// +kubebuilder:validation:MaxLength=57
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// Description provides a short explanation of what the driver does.
	// +optional
	Description string `json:"description,omitempty"`
	// URL defines the location of the driver binary that will
	// be downloaded when the driver is enabled. This can either be
	// an absolute url to a remote resource, or a reference to localhost.
	// +required
	URL string `json:"url" norman:"required"`
	// ExternalID is not currently used.
	// +optional
	ExternalID string `json:"externalId,omitempty"`
	// Builtin specifies if a driver is built into Rancher via rancher-machine.
	// +optional
	Builtin bool `json:"builtin,omitempty"`
	// Active specifies if the driver can be used to provision clusters.
	// +required
	Active bool `json:"active"`
	// AddCloudCredential determines if an associated cloud credential
	// dynamic schema should be created for this driver even if it is not
	// enabled.
	// +optional
	AddCloudCredential bool `json:"addCloudCredential,omitempty"`
	// Checksum is used to represent the expected content of the driver
	// binary. When this value changes, the driver binary will be
	// redownloaded from its URL.
	// +kubebuilder:validation:Pattern="^$|^[a-fA-F0-9]{32,128}$"
	// +kubebuilder:validation:MaxLength=128
	// +optional
	Checksum string `json:"checksum,omitempty"`
	// UIURL is the url to load for a customized Add Nodes screen for this driver.
	// +optional
	UIURL string `json:"uiUrl,omitempty"`
	// WhitelistDomains is a list of domains which will be automatically
	// white-listed by Rancher to allow for the driver to be downloaded.
	// +optional
	WhitelistDomains []string `json:"whitelistDomains,omitempty"`
}

type PublicEndpoint struct {
	NodeName  string   `json:"nodeName,omitempty" norman:"type=reference[/v3/schemas/node],nocreate,noupdate"`
	Addresses []string `json:"addresses,omitempty" norman:"nocreate,noupdate"`
	Port      int32    `json:"port,omitempty" norman:"nocreate,noupdate"`
	Protocol  string   `json:"protocol,omitempty" norman:"nocreate,noupdate"`
	// for node port service endpoint
	ServiceName string `json:"serviceName,omitempty" norman:"type=reference[service],nocreate,noupdate"`
	// for host port endpoint
	PodName string `json:"podName,omitempty" norman:"type=reference[pod],nocreate,noupdate"`
	// for ingress endpoint. ServiceName, podName, ingressName are mutually exclusive
	IngressName string `json:"ingressName,omitempty" norman:"type=reference[ingress],nocreate,noupdate"`
	// Hostname/path are set for Ingress endpoints
	Hostname string `json:"hostname,omitempty" norman:"nocreate,noupdate"`
	Path     string `json:"path,omitempty" norman:"nocreate,noupdate"`
	// True when endpoint is exposed on every node
	AllNodes bool `json:"allNodes" norman:"nocreate,noupdate"`
}

type NodeDrainInput = rketypes.NodeDrainInput

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CloudCredential struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CloudCredentialSpec `json:"spec"`
}

type CloudCredentialSpec struct {
	DisplayName        string              `json:"displayName"`
	Description        string              `json:"description,omitempty"`
	S3CredentialConfig *S3CredentialConfig `json:"s3credentialConfig,omitempty"`
}

type S3CredentialConfig struct {
	AccessKey            string `norman:"required"`
	SecretKey            string `norman:"required,type=password"`
	DefaultRegion        string
	DefaultEndpoint      string
	DefaultEndpointCA    string
	DefaultSkipSSLVerify string
	DefaultBucket        string
	DefaultFolder        string
}
