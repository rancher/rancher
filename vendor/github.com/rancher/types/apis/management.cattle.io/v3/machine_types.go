package v3

import (
	"time"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	Conditions         []NodeCondition   `json:"conditions,omitempty"`
	InternalNodeStatus v1.NodeStatus     `json:"internalNodeStatus,omitempty"`
	NodeName           string            `json:"nodeName,omitempty"`
	Requested          v1.ResourceList   `json:"requested,omitempty"`
	Limits             v1.ResourceList   `json:"limits,omitempty"`
	NodeTemplateSpec   *NodeTemplateSpec `json:"nodeTemplateSpec,omitempty"`
	NodeConfig         *RKEConfigNode    `json:"rkeNode,omitempty"`
	NodeAnnotations    map[string]string `json:"nodeAnnotations,omitempty"`
	NodeLabels         map[string]string `json:"nodeLabels,omitempty"`
	NodeTaints         []v1.Taint        `json:"nodeTaints,omitempty"`
	DockerInfo         *DockerInfo       `json:"dockerInfo,omitempty"`
	NodePlan           *NodePlan         `json:"nodePlan,omitempty"`
	AppliedNodeVersion int               `json:"appliedNodeVersion,omitempty"`
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
	DockerRootDir      string
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

	HostnamePrefix  string            `json:"hostnamePrefix" norman:"required,notnullable"`
	Quantity        int               `json:"quantity" norman:"required,default=1"`
	NodeLabels      map[string]string `json:"nodeLabels"`
	NodeAnnotations map[string]string `json:"nodeAnnotations"`
	NodeTaints      []v1.Taint        `json:"nodeTaints,omitempty"`

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
}

type NodePlan struct {
	Plan    *RKEConfigNodePlan `json:"plan,omitempty"`
	Version int                `json:"version,omitempty"`
	// current default in rancher-agent is 2m (120s)
	AgentCheckInterval int `json:"agentCheckInterval,omitempty" norman:"min=1,max=1800,default=120"`
}

type NodeCommonParams struct {
	AuthCertificateAuthority string            `json:"authCertificateAuthority,omitempty"`
	AuthKey                  string            `json:"authKey,omitempty"`
	EngineInstallURL         string            `json:"engineInstallURL,omitempty"`
	DockerVersion            string            `json:"dockerVersion,omitempty"`
	EngineOpt                map[string]string `json:"engineOpt,omitempty"`
	EngineInsecureRegistry   []string          `json:"engineInsecureRegistry,omitempty"`
	EngineRegistryMirror     []string          `json:"engineRegistryMirror,omitempty"`
	EngineLabel              map[string]string `json:"engineLabel,omitempty"`
	EngineStorageDriver      string            `json:"engineStorageDriver,omitempty"`
	EngineEnv                map[string]string `json:"engineEnv,omitempty"`
	UseInternalIPAddress     bool              `json:"useInternalIpAddress,omitempty" norman:"default=true,noupdate"`
}

type NodeDriver struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec NodeDriverSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status NodeDriverStatus `json:"status"`
}

type NodeDriverStatus struct {
	Conditions                  []Condition `json:"conditions"`
	AppliedURL                  string      `json:"appliedURL"`
	AppliedChecksum             string      `json:"appliedChecksum"`
	AppliedDockerMachineVersion string      `json:"appliedDockerMachineVersion"`
}

var (
	NodeDriverConditionDownloaded condition.Cond = "Downloaded"
	NodeDriverConditionInstalled  condition.Cond = "Installed"
	NodeDriverConditionActive     condition.Cond = "Active"
	NodeDriverConditionInactive   condition.Cond = "Inactive"
)

type Condition struct {
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
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type NodeDriverSpec struct {
	DisplayName      string   `json:"displayName"`
	Description      string   `json:"description"`
	URL              string   `json:"url" norman:"required"`
	ExternalID       string   `json:"externalId"`
	Builtin          bool     `json:"builtin"`
	Active           bool     `json:"active"`
	Checksum         string   `json:"checksum"`
	UIURL            string   `json:"uiUrl"`
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

type NodeDrainInput struct {
	// Drain node even if there are pods not managed by a ReplicationController, Job, or DaemonSet
	// Drain will not proceed without Force set to true if there are such pods
	Force bool `yaml:"force" json:"force,omitempty"`
	// If there are DaemonSet-managed pods, drain will not proceed without IgnoreDaemonSets set to true
	// (even when set to true, kubectl won't delete pods - so setting default to true)
	IgnoreDaemonSets bool `yaml:"ignore_daemonsets" json:"ignoreDaemonSets,omitempty" norman:"default=true"`
	// Continue even if there are pods using emptyDir
	DeleteLocalData bool `yaml:"delete_local_data" json:"deleteLocalData,omitempty"`
	//Period of time in seconds given to each pod to terminate gracefully.
	// If negative, the default value specified in the pod will be used
	GracePeriod int `yaml:"grace_period" json:"gracePeriod,omitempty" norman:"default=-1"`
	// Time to wait (in seconds) before giving up for one try
	Timeout int `yaml:"timeout" json:"timeout" norman:"min=1,max=10800,default=120"`
}

type CloudCredential struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CloudCredentialSpec `json:"spec"`
}

type CloudCredentialSpec struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}
