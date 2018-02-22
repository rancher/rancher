package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
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
	DisplayName      string `json:"displayName"`
	Description      string `json:"description"`
	Driver           string `json:"driver" norman:"nocreate,noupdate"`
	NodeCommonParams `json:",inline"`
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
}

var (
	NodeConditionInitialized condition.Cond = "Initialized"
	NodeConditionProvisioned condition.Cond = "Provisioned"
	NodeConditionRegistered  condition.Cond = "Registered"
	NodeConditionRemoved     condition.Cond = "Removed"
	NodeConditionConfigSaved condition.Cond = "Saved"
	NodeConditionReady       condition.Cond = "Ready"
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

type NodePoolSpec struct {
	Etcd             bool   `json:"etcd"`
	ControlPlane     bool   `json:"controlPlane"`
	Worker           bool   `json:"worker"`
	NodeTemplateName string `json:"nodeTemplateName,omitempty" norman:"type=reference[nodeTemplate],required,notnullable"`

	HostnamePrefix  string            `json:"hostnamePrefix" norman:"required,notnullable"`
	Quantity        int               `json:"quantity" norman:"required,default=1"`
	NodeLabels      map[string]string `json:"nodeLabels"`
	NodeAnnotations map[string]string `json:"nodeAnnotations"`

	DisplayName string `json:"displayName"`
	ClusterName string `json:"clusterName,omitempty" norman:"type=reference[cluster],noupdate,required"`
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
	SSHKey string `yaml:"ssh_key" json:"sshKey,omitempty"`
}

type NodeSpec struct {
	// Common fields.  They aren't in a shared struct because the annotations are different

	Etcd             bool   `json:"etcd" norman:"noupdate"`
	ControlPlane     bool   `json:"controlPlane" norman:"noupdate"`
	Worker           bool   `json:"worker" norman:"noupdate"`
	NodeTemplateName string `json:"nodeTemplateName,omitempty" norman:"type=reference[nodeTemplate],noupdate"`

	NodePoolName      string        `json:"nodePoolName" norman:"type=reference[nodePool],nocreate,noupdate"`
	CustomConfig      *CustomConfig `json:"customConfig"`
	Imported          bool          `json:"imported"`
	Description       string        `json:"description,omitempty"`
	DisplayName       string        `json:"displayName"`
	RequestedHostname string        `json:"requestedHostname,omitempty" norman:"type=dnsLabel,nullable,noupdate,required"`
	ClusterName       string        `json:"clusterName,omitempty" norman:"type=reference[cluster],noupdate,required"`
	InternalNodeSpec  v1.NodeSpec   `json:"internalNodeSpec"`
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
	Conditions []Condition `json:"conditions"`
}

var (
	NodeDriverConditionDownloaded condition.Cond = "Downloaded"
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
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	URL         string `json:"url" norman:"required"`
	ExternalID  string `json:"externalId"`
	Builtin     bool   `json:"builtin"`
	Active      bool   `json:"active"`
	Checksum    string `json:"checksum"`
	UIURL       string `json:"uiUrl"`
}
