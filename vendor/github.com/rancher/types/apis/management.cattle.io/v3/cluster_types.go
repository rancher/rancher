package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

type ClusterConditionType string

const (
	// ClusterConditionReady Cluster ready to serve API (healthy when true, unhealthy when false)
	ClusterConditionReady          condition.Cond = "Ready"
	ClusterConditionPending        condition.Cond = "Pending"
	ClusterConditionCertsGenerated condition.Cond = "CertsGenerated"
	ClusterConditionEtcd           condition.Cond = "etcd"
	ClusterConditionProvisioned    condition.Cond = "Provisioned"
	ClusterConditionUpdated        condition.Cond = "Updated"
	ClusterConditionWaiting        condition.Cond = "Waiting"
	ClusterConditionRemoved        condition.Cond = "Removed"
	// ClusterConditionNoDiskPressure true when all cluster nodes have sufficient disk
	ClusterConditionNoDiskPressure condition.Cond = "NoDiskPressure"
	// ClusterConditionNoMemoryPressure true when all cluster nodes have sufficient memory
	ClusterConditionNoMemoryPressure condition.Cond = "NoMemoryPressure"
	// ClusterConditionconditionDefaultProjectCreated true when default project has been created
	ClusterConditionconditionDefaultProjectCreated condition.Cond = "DefaultProjectCreated"
	// ClusterConditionconditionSystemProjectCreated true when system project has been created
	ClusterConditionconditionSystemProjectCreated condition.Cond = "SystemProjectCreated"
	// ClusterConditionDefaultNamespaceAssigned true when cluster's default namespace has been initially assigned
	ClusterConditionDefaultNamespaceAssigned condition.Cond = "DefaultNamespaceAssigned"
	// ClusterConditionSystemNamespacesAssigned true when cluster's system namespaces has been initially assigned to
	// a system project
	ClusterConditionSystemNamespacesAssigned condition.Cond = "SystemNamespacesAssigned"
	ClusterConditionAddonDeploy              condition.Cond = "AddonDeploy"
	ClusterConditionSystemAccountCreated     condition.Cond = "SystemAccountCreated"
	ClusterConditionAgentDeployed            condition.Cond = "AgentDeployed"
	ClusterConditionGlobalAdminsSynced       condition.Cond = "GlobalAdminsSynced"
	ClusterConditionInitialRolesPopulated    condition.Cond = "InitialRolesPopulated"

	ClusterDriverImported = "imported"
	ClusterDriverLocal    = "local"
	ClusterDriverRKE      = "rancherKubernetesEngine"
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
	DisplayName                          string                               `json:"displayName" norman:"required"`
	Description                          string                               `json:"description"`
	Internal                             bool                                 `json:"internal" norman:"nocreate,noupdate"`
	DesiredAgentImage                    string                               `json:"desiredAgentImage"`
	ImportedConfig                       *ImportedConfig                      `json:"importedConfig,omitempty" norman:"nocreate,noupdate"`
	GoogleKubernetesEngineConfig         *GoogleKubernetesEngineConfig        `json:"googleKubernetesEngineConfig,omitempty"`
	AzureKubernetesServiceConfig         *AzureKubernetesServiceConfig        `json:"azureKubernetesServiceConfig,omitempty"`
	RancherKubernetesEngineConfig        *RancherKubernetesEngineConfig       `json:"rancherKubernetesEngineConfig,omitempty"`
	AmazonElasticContainerServiceConfig  *AmazonElasticContainerServiceConfig `json:"amazonElasticContainerServiceConfig,omitempty"`
	DefaultPodSecurityPolicyTemplateName string                               `json:"defaultPodSecurityPolicyTemplateName,omitempty" norman:"type=reference[podSecurityPolicyTemplate]"`
	DefaultClusterRoleForProjectMembers  string                               `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
	DockerRootDir                        string                               `json:"dockerRootDir,omitempty" norman:"default=/var/lib/docker"`
	EnableNetworkPolicy                  *bool                                `json:"enableNetworkPolicy" norman:"nocreate,noupdate,default=false"`
}

type ImportedConfig struct {
	KubeConfig string `json:"kubeConfig" norman:"type=password"`
}

type ClusterStatus struct {
	//Conditions represent the latest available observations of an object's current state:
	//More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	//Component statuses will represent cluster's components (etcd/controller/scheduler) health
	// https://kubernetes.io/docs/api-reference/v1.8/#componentstatus-v1-core
	Driver                               string                   `json:"driver"`
	AgentImage                           string                   `json:"agentImage"`
	ComponentStatuses                    []ClusterComponentStatus `json:"componentStatuses,omitempty"`
	APIEndpoint                          string                   `json:"apiEndpoint,omitempty"`
	ServiceAccountToken                  string                   `json:"serviceAccountToken,omitempty"`
	CACert                               string                   `json:"caCert,omitempty"`
	Capacity                             v1.ResourceList          `json:"capacity,omitempty"`
	Allocatable                          v1.ResourceList          `json:"allocatable,omitempty"`
	AppliedSpec                          ClusterSpec              `json:"appliedSpec,omitempty"`
	FailedSpec                           *ClusterSpec             `json:"failedSpec,omitempty"`
	Requested                            v1.ResourceList          `json:"requested,omitempty"`
	Limits                               v1.ResourceList          `json:"limits,omitempty"`
	Version                              *version.Info            `json:"version,omitempty"`
	AppliedPodSecurityPolicyTemplateName string                   `json:"appliedPodSecurityPolicyTemplateId"`
	AppliedEnableNetworkPolicy           bool                     `json:"appliedEnableNetworkPolicy" norman:"nocreate,noupdate,default=false"`
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
	ProjectID string `json:"projectId,omitempty" norman:"required"`
	// The zone to launch the cluster
	Zone string `json:"zone,omitempty" norman:"required"`
	// The IP address range of the container pods
	ClusterIpv4Cidr string `json:"clusterIpv4Cidr,omitempty"`
	// An optional description of this cluster
	Description string `json:"description,omitempty"`
	// The number of nodes in this cluster
	NodeCount int64 `json:"nodeCount,omitempty" norman:"required"`
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
	Credential string `json:"credential,omitempty" norman:"required,type=password"`
	// Enable alpha feature
	EnableAlphaFeature bool `json:"enableAlphaFeature,omitempty"`
	// Configuration for the HTTP (L7) load balancing controller addon
	DisableHTTPLoadBalancing bool `json:"disableHttpLoadBalancing,omitempty"`
	// Configuration for the horizontal pod autoscaling feature, which increases or decreases the number of replica pods a replication controller has based on the resource usage of the existing pods
	DisableHorizontalPodAutoscaling bool `json:"disableHorizontalPodAutoscaling,omitempty"`
	// Configuration for the Kubernetes Dashboard
	EnableKubernetesDashboard bool `json:"enableKubernetesDashboard,omitempty"`
	// Configuration for NetworkPolicy
	DisableNetworkPolicyConfig bool `json:"disableNetworkPolicyConfig,omitempty"`
	// The list of Google Compute Engine locations in which the cluster's nodes should be located
	Locations []string `json:"locations,omitempty"`
	// Image Type
	ImageType string `json:"imageType,omitempty"`
	// Network
	Network string `json:"network,omitempty"`
	// Sub Network
	SubNetwork string `json:"subNetwork,omitempty"`
	// Configuration for LegacyAbac
	EnableLegacyAbac        bool   `json:"enableLegacyAbac,omitempty"`
	NoStackdriverLogging    bool   `json:"noStackdriverLogging"`
	NoStackdriverMonitoring bool   `json:"noStackdriverMonitoring"`
	NoNetworkPolicy         bool   `json:"noNetworkPolicy"`
	MaintenanceWindow       string `json:"maintenanceWindow"`
}

type AzureKubernetesServiceConfig struct {
	// Subscription credentials which uniquely identify Microsoft Azure subscription. The subscription ID forms part of the URI for every service call.
	SubscriptionID string `json:"subscriptionId,omitempty" norman:"required"`
	// The name of the resource group.
	ResourceGroup string `json:"resourceGroup,omitempty" norman:"required"`
	// Resource location
	Location string `json:"location,omitempty"`
	// Resource tags
	Tag map[string]string `json:"tags,omitempty"`
	// Number of agents (VMs) to host docker containers. Allowed values must be in the range of 1 to 100 (inclusive). The default value is 1.
	Count int64 `json:"count,omitempty"`
	// DNS prefix to be used to create the FQDN for the agent pool.
	AgentDNSPrefix string `json:"agentDnsPrefix,,omitempty"`
	// FDQN for the agent pool
	AgentPoolName string `json:"agentPoolName,,omitempty"`
	// OS Disk Size in GB to be used to specify the disk size for every machine in this master/agent pool. If you specify 0, it will apply the default osDisk size according to the vmSize specified.
	OsDiskSizeGB int64 `json:"osDiskSizeGb,omitempty"`
	// Size of agent VMs
	AgentVMSize string `json:"agentVmSize,omitempty"`
	// Version of Kubernetes specified when creating the managed cluster
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// Path to the public key to use for SSH into cluster
	SSHPublicKeyContents string `json:"sshPublicKeyContents,omitempty" norman:"required"`
	// Kubernetes Master DNS prefix (must be unique within Azure)
	MasterDNSPrefix string `json:"masterDnsPrefix,omitempty"`
	// Kubernetes admin username
	AdminUsername string `json:"adminUsername,omitempty"`
	// Different Base URL if required, usually needed for testing purposes
	BaseURL string `json:"baseUrl,omitempty"`
	// Azure Client ID to use
	ClientID string `json:"clientId,omitempty" norman:"required"`
	// Tenant ID to create the cluster under
	TenantID string `json:"tenantId,omitempty" norman:"required"`
	// Secret associated with the Client ID
	ClientSecret string `json:"clientSecret,omitempty" norman:"required,type=password"`
	// Virtual network to use for the AKS cluster
	VirtualNetwork string `json:"virtualNetwork,omitempty"`
	// Subnet to use for the AKS Cluster (must be within the virtual network)
	Subnet string `json:"subnet,omitempty"`
	// The resource group that the virtual network is in.  If omited it is assumed to match the resource group of the cluster
	VirtualNetworkResourceGroup string `json:"virtualNetworkResourceGroup,omitempty"`
}

type AmazonElasticContainerServiceConfig struct {
	AccessKey string `json:"accessKey" norman:"required"`
	SecretKey string `json:"secretKey" norman:"required,type=password"`

	Region         string   `json:"region"`
	InstanceType   string   `json:"instanceType"`
	MinimumNodes   int      `json:"minimumNodes"`
	MaximumNodes   int      `json:"maximumNodes"`
	VirtualNetwork string   `json:"virtualNetwork,omitempty"`
	Subnets        []string `json:"subnets,omitempty"`
	SecurityGroups []string `json:"securityGroups,omitempty"`
	ServiceRole    string   `json:"serviceRole,omitempty"`
}

type ClusterEvent struct {
	types.Namespaced
	v1.Event
	ClusterName string `json:"clusterName" norman:"type=reference[cluster]"`
}

type ClusterRegistrationToken struct {
	types.Namespaced

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
	ClusterName string `json:"clusterName" norman:"required,type=reference[cluster]"`
}

type ClusterRegistrationTokenStatus struct {
	InsecureCommand string `json:"insecureCommand"`
	Command         string `json:"command"`
	NodeCommand     string `json:"nodeCommand"`
	ManifestURL     string `json:"manifestUrl"`
	Token           string `json:"token"`
}

type GenerateKubeConfigOutput struct {
	Config string `json:"config"`
}

type ExportOutput struct {
	YAMLOutput string `json:"yamlOutput"`
}

type ImportClusterYamlInput struct {
	YAML             string `json:"yaml,omitempty"`
	DefaultNamespace string `json:"defaultNamespace,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	ProjectName      string `json:"projectName,omitempty" norman:"type=reference[project]"`
}

type ImportYamlOutput struct {
	Message string `json:"message,omitempty"`
}
