package v3

import (
	"github.com/rancher/norman/condition"
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
	Conditions          []MachineCondition   `json:"conditions"`
	NodeStatus          v1.NodeStatus        `json:"nodeStatus"`
	NodeName            string               `json:"nodeName"`
	Requested           v1.ResourceList      `json:"requested,omitempty"`
	Limits              v1.ResourceList      `json:"limits,omitempty"`
	MachineTemplateSpec *MachineTemplateSpec `json:"machineTemplateSpec"`
	NodeConfig          *RKEConfigNode       `json:"rkeNode"`
	SSHUser             string               `json:"sshUser"`
	MachineDriverConfig string               `json:"machineDriverConfig"`
}

var (
	MachineConditionInitialized condition.Cond = "Initialized"
	MachineConditionProvisioned condition.Cond = "Provisioned"
	MachineConditionConfigSaved condition.Cond = "Saved"
	MachineConditionConfigReady condition.Cond = "Ready"
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
}

type MachineSpec struct {
	NodeSpec            v1.NodeSpec `json:"nodeSpec"`
	DisplayName         string      `json:"displayName"`
	ClusterName         string      `json:"clusterName" norman:"type=reference[cluster]"`
	Roles               []string    `json:"roles"`
	MachineTemplateName string      `json:"machineTemplateName" norman:"type=reference[machineTemplate]"`
	Description         string      `json:"description"`
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
	Description string `json:"description"`
	URL         string `json:"url"`
	ExternalID  string `json:"externalId"`
	Builtin     bool   `json:"builtin"`
	Active      bool   `json:"active"`
	Checksum    string `json:"checksum"`
	UIURL       string `json:"uiUrl"`
}
