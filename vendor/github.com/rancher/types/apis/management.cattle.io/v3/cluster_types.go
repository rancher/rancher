package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterConditionType string

const (
	// ClusterConditionReady Cluster ready to serve API (healthy when true, unhealthy when false)
	ClusterConditionReady           condition.Cond = "Ready"
	ClusterConditionMachinesCreated condition.Cond = "MachinesCreated"
	// ClusterConditionProvisioned Cluster is provisioned
	ClusterConditionProvisioned condition.Cond = "Provisioned"
	ClusterConditionUpdated     condition.Cond = "Updated"
	ClusterConditionRemoved     condition.Cond = "Removed"
	// ClusterConditionNoDiskPressure true when all cluster nodes have sufficient disk
	ClusterConditionNoDiskPressure condition.Cond = "NoDiskPressure"
	// ClusterConditionNoMemoryPressure true when all cluster nodes have sufficient memory
	ClusterConditionNoMemoryPressure condition.Cond = "NoMemoryPressure"
	// ClusterConditionconditionDefautlProjectCreated true when default project has been created
	ClusterConditionconditionDefautlProjectCreated condition.Cond = "DefaultProjectCreated"
	// ClusterConditionDefaultNamespaceAssigned true when cluster's default namespace has been initially assigned
	ClusterConditionDefaultNamespaceAssigned condition.Cond = "DefaultNamespaceAssigned"
	// More conditions can be added if unredlying controllers request it
)

type Cluster struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec ClusterSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status ClusterStatus `json:"status"`
}

type ClusterSpec struct {
	Nodes                                []MachineConfig                `json:"nodes"`
	DisplayName                          string                         `json:"displayName"`
	Description                          string                         `json:"description"`
	Internal                             bool                           `json:"internal" norman:"nocreate,noupdate"`
	GoogleKubernetesEngineConfig         *GoogleKubernetesEngineConfig  `json:"googleKubernetesEngineConfig,omitempty"`
	AzureKubernetesServiceConfig         *AzureKubernetesServiceConfig  `json:"azureKubernetesServiceConfig,omitempty"`
	RancherKubernetesEngineConfig        *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
	DefaultPodSecurityPolicyTemplateName string                         `json:"defaultPodSecurityPolicyTemplateName,omitempty" norman:"type=reference[podSecurityPolicyTemplate]"`
	DefaultClusterRoleForProjectMembers  string                         `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
}

type ClusterStatus struct {
	//Conditions represent the latest available observations of an object's current state:
	//More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	//Component statuses will represent cluster's components (etcd/controller/scheduler) health
	// https://kubernetes.io/docs/api-reference/v1.8/#componentstatus-v1-core
	Driver              string                   `json:"driver"`
	ComponentStatuses   []ClusterComponentStatus `json:"componentStatuses,omitempty"`
	APIEndpoint         string                   `json:"apiEndpoint,omitempty"`
	ServiceAccountToken string                   `json:"serviceAccountToken,omitempty"`
	CACert              string                   `json:"caCert,omitempty"`
	Capacity            v1.ResourceList          `json:"capacity,omitempty"`
	Allocatable         v1.ResourceList          `json:"allocatable,omitempty"`
	AppliedSpec         ClusterSpec              `json:"appliedSpec,omitempty"`
	Requested           v1.ResourceList          `json:"requested,omitempty"`
	Limits              v1.ResourceList          `json:"limits,omitempty"`
}

type ClusterComponentStatus struct {
	Name       string                  `json:"name"`
	Conditions []v1.ComponentCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
}

type ClusterCondition struct {
	// Type of cluster condition.
	Type ClusterConditionType `json:"type"`
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

type GoogleKubernetesEngineConfig struct {
	// ProjectID is the ID of your project to use when creating a cluster
	ProjectID string `json:"projectId,omitempty"`
	// The zone to launch the cluster
	Zone string `json:"zone,omitempty"`
	// The IP address range of the container pods
	ClusterIpv4Cidr string `json:"clusterIpv4Cidr,omitempty"`
	// An optional description of this cluster
	Description string `json:"description,omitempty"`
	// The number of nodes in this cluster
	NodeCount int64 `json:"nodeCount,omitempty"`
	// Size of the disk attached to each node
	DiskSizeGb int64 `json:"diskSizeGb,omitempty"`
	// The name of a Google Compute Engine
	MachineType string `json:"machineType,omitempty"`
	// Node kubernetes version
	NodeVersion string `json:"nodeVersion,omitempty"`
	// the master kubernetes version
	MasterVersion string `json:"masterVersion,omitempty"`
	// The map of Kubernetes labels (key/value pairs) to be applied
	// to each node.
	Labels map[string]string `json:"labels,omitempty"`
	// The content of the credential file(key.json)
	Credential string `json:"credential,omitempty"`
	// Enable alpha feature
	EnableAlphaFeature bool `json:"enableAlphaFeature,omitempty"`
	// Configuration for the HTTP (L7) load balancing controller addon
	HTTPLoadBalancing bool `json:"httpLoadBalancing,omitempty"`
	// Configuration for the horizontal pod autoscaling feature, which increases or decreases the number of replica pods a replication controller has based on the resource usage of the existing pods
	HorizontalPodAutoscaling bool `json:"horizontalPodAutoscaling,omitempty"`
	// Configuration for the Kubernetes Dashboard
	KubernetesDashboard bool `json:"kubernetesDashboard,omitempty"`
	// Configuration for NetworkPolicy
	NetworkPolicyConfig bool `json:"networkPolicyConfig,omitempty"`
	// The list of Google Compute Engine locations in which the cluster's nodes should be located
	Locations []string `json:"locations,omitempty"`
	// Image Type
	ImageType string `json:"imageType,omitempty"`
	// Network
	Network string `json:"network,omitempty"`
	// Sub Network
	SubNetwork string `json:"subNetwork,omitempty"`
	// Configuration for LegacyAbac
	LegacyAbac bool `json:"legacyAbac,omitempty"`
}

type AzureKubernetesServiceConfig struct {
	//TBD
}

type ClusterEvent struct {
	types.Namespaced
	v1.Event
	ClusterName string `json:"clusterName" norman:"type=reference[cluster]"`
}

type ClusterRegistrationToken struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec ClusterRegistrationTokenSpec `json:"spec"`
	// Most recent observed status of the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status ClusterRegistrationTokenStatus `json:"status"`
}

type ClusterRegistrationTokenSpec struct {
}

type ClusterRegistrationTokenStatus struct {
	Command     string `json:"command"`
	ManifestURL string `json:"manifestUrl"`
	Token       string `json:"token"`
}
