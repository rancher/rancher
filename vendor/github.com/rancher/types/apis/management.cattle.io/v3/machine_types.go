package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
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
	DisplayName         string `json:"displayName"`
	Description         string `json:"description"`
	Driver              string `json:"driver"`
	MachineCommonParams `json:",inline"`
}

type Machine struct {
	types.Namespaced

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
	Conditions          []MachineCondition   `json:"conditions,omitempty"`
	NodeStatus          v1.NodeStatus        `json:"nodeStatus,omitempty"`
	NodeName            string               `json:"nodeName,omitempty"`
	Requested           v1.ResourceList      `json:"requested,omitempty"`
	Limits              v1.ResourceList      `json:"limits,omitempty"`
	MachineTemplateSpec *MachineTemplateSpec `json:"machineTemplateSpec,omitempty"`
	NodeConfig          *RKEConfigNode       `json:"rkeNode,omitempty"`
	SSHUser             string               `json:"sshUser,omitempty"`
	MachineDriverConfig string               `json:"machineDriverConfig,omitempty"`
	NodeAnnotations     map[string]string    `json:"nodeAnnotations,omitempty"`
	NodeLabels          map[string]string    `json:"nodeLabels,omitempty"`
	NodeTaints          []v1.Taint           `json:"nodeTaints,omitempty"`
}

var (
	MachineConditionInitialized condition.Cond = "Initialized"
	MachineConditionProvisioned condition.Cond = "Provisioned"
	MachineConditionConfigSaved condition.Cond = "Saved"
	MachineConditionReady       condition.Cond = "Ready"
)

type MachineCondition struct {
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

type MachineConfig struct {
	MachineSpec
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
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

type MachineSpec struct {
	NodeSpec             v1.NodeSpec   `json:"nodeSpec"`
	CustomConfig         *CustomConfig `json:"customConfig"`
	Imported             bool          `json:"imported"`
	Description          string        `json:"description,omitempty"`
	DisplayName          string        `json:"displayName"`
	RequestedHostname    string        `json:"requestedHostname,omitempty" norman:"type=dnsLabel,nullable,noupdate,required"`
	ClusterName          string        `json:"clusterName,omitempty" norman:"type=reference[cluster],noupdate,required"`
	Role                 []string      `json:"role,omitempty" norman:"noupdate,type=array[enum],options=etcd|worker|controlplane"`
	MachineTemplateName  string        `json:"machineTemplateName,omitempty" norman:"type=reference[machineTemplate],noupdate"`
	UseInternalIPAddress bool          `json:"useInternalIpAddress,omitempty" norman:"default=true,noupdate"`
}

type MachineCommonParams struct {
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

var (
	MachineDriverConditionDownloaded condition.Cond = "Downloaded"
	MachineDriverConditionActive     condition.Cond = "Active"
	MachineDriverConditionInactive   condition.Cond = "Inactive"
)

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
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type MachineDriverSpec struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	URL         string `json:"url" norman:"required"`
	ExternalID  string `json:"externalId"`
	Builtin     bool   `json:"builtin"`
	Active      bool   `json:"active"`
	Checksum    string `json:"checksum"`
	UIURL       string `json:"uiUrl"`
}
