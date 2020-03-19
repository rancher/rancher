package v3

import (
	"bytes"
	"encoding/gob"
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

type ClusterConditionType string

const (
	ClusterActionGenerateKubeconfig    = "generateKubeconfig"
	ClusterActionImportYaml            = "importYaml"
	ClusterActionExportYaml            = "exportYaml"
	ClusterActionViewMonitoring        = "viewMonitoring"
	ClusterActionEditMonitoring        = "editMonitoring"
	ClusterActionEnableMonitoring      = "enableMonitoring"
	ClusterActionDisableMonitoring     = "disableMonitoring"
	ClusterActionBackupEtcd            = "backupEtcd"
	ClusterActionRestoreFromEtcdBackup = "restoreFromEtcdBackup"
	ClusterActionRotateCertificates    = "rotateCertificates"
	ClusterActionRunSecurityScan       = "runSecurityScan"
	ClusterActionSaveAsTemplate        = "saveAsTemplate"

	// ClusterConditionReady Cluster ready to serve API (healthy when true, unhealthy when false)
	ClusterConditionReady          condition.Cond = "Ready"
	ClusterConditionPending        condition.Cond = "Pending"
	ClusterConditionCertsGenerated condition.Cond = "CertsGenerated"
	ClusterConditionEtcd           condition.Cond = "etcd"
	ClusterConditionProvisioned    condition.Cond = "Provisioned"
	ClusterConditionUpdated        condition.Cond = "Updated"
	ClusterConditionUpgraded       condition.Cond = "Upgraded"
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
	ClusterConditionSystemNamespacesAssigned   condition.Cond = "SystemNamespacesAssigned"
	ClusterConditionAddonDeploy                condition.Cond = "AddonDeploy"
	ClusterConditionSystemAccountCreated       condition.Cond = "SystemAccountCreated"
	ClusterConditionAgentDeployed              condition.Cond = "AgentDeployed"
	ClusterConditionGlobalAdminsSynced         condition.Cond = "GlobalAdminsSynced"
	ClusterConditionInitialRolesPopulated      condition.Cond = "InitialRolesPopulated"
	ClusterConditionServiceAccountMigrated     condition.Cond = "ServiceAccountMigrated"
	ClusterConditionPrometheusOperatorDeployed condition.Cond = "PrometheusOperatorDeployed"
	ClusterConditionMonitoringEnabled          condition.Cond = "MonitoringEnabled"
	ClusterConditionAlertingEnabled            condition.Cond = "AlertingEnabled"

	ClusterDriverImported = "imported"
	ClusterDriverLocal    = "local"
	ClusterDriverRKE      = "rancherKubernetesEngine"
	ClusterDriverK3s      = "k3s"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

type ClusterSpecBase struct {
	DesiredAgentImage                    string                         `json:"desiredAgentImage"`
	DesiredAuthImage                     string                         `json:"desiredAuthImage"`
	AgentImageOverride                   string                         `json:"agentImageOverride"`
	RancherKubernetesEngineConfig        *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
	DefaultPodSecurityPolicyTemplateName string                         `json:"defaultPodSecurityPolicyTemplateName,omitempty" norman:"type=reference[podSecurityPolicyTemplate]"`
	DefaultClusterRoleForProjectMembers  string                         `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
	DockerRootDir                        string                         `json:"dockerRootDir,omitempty" norman:"default=/var/lib/docker"`
	EnableNetworkPolicy                  *bool                          `json:"enableNetworkPolicy" norman:"default=false"`
	EnableClusterAlerting                bool                           `json:"enableClusterAlerting" norman:"default=false"`
	EnableClusterMonitoring              bool                           `json:"enableClusterMonitoring" norman:"default=false"`
	WindowsPreferedCluster               bool                           `json:"windowsPreferedCluster" norman:"noupdate"`
	LocalClusterAuthEndpoint             LocalClusterAuthEndpoint       `json:"localClusterAuthEndpoint,omitempty"`
	ScheduledClusterScan                 *ScheduledClusterScan          `json:"scheduledClusterScan,omitempty"`
}

type ClusterSpec struct {
	ClusterSpecBase
	DisplayName                         string              `json:"displayName" norman:"required"`
	Description                         string              `json:"description"`
	Internal                            bool                `json:"internal" norman:"nocreate,noupdate"`
	K3sConfig                           *K3sConfig          `json:"k3sConfig,omitempty"`
	ImportedConfig                      *ImportedConfig     `json:"importedConfig,omitempty" norman:"nocreate,noupdate"`
	GoogleKubernetesEngineConfig        *MapStringInterface `json:"googleKubernetesEngineConfig,omitempty"`
	AzureKubernetesServiceConfig        *MapStringInterface `json:"azureKubernetesServiceConfig,omitempty"`
	AmazonElasticContainerServiceConfig *MapStringInterface `json:"amazonElasticContainerServiceConfig,omitempty"`
	GenericEngineConfig                 *MapStringInterface `json:"genericEngineConfig,omitempty"`
	ClusterTemplateName                 string              `json:"clusterTemplateName,omitempty" norman:"type=reference[clusterTemplate],nocreate,noupdate"`
	ClusterTemplateRevisionName         string              `json:"clusterTemplateRevisionName,omitempty" norman:"type=reference[clusterTemplateRevision]"`
	ClusterTemplateAnswers              Answer              `json:"answers,omitempty"`
	ClusterTemplateQuestions            []Question          `json:"questions,omitempty" norman:"nocreate,noupdate"`
}

type ImportedConfig struct {
	KubeConfig string `json:"kubeConfig" norman:"type=password"`
}

type ClusterStatus struct {
	// Conditions represent the latest available observations of an object's current state:
	// More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	// Component statuses will represent cluster's components (etcd/controller/scheduler) health
	// https://kubernetes.io/docs/api-reference/v1.8/#componentstatus-v1-core
	Driver                               string                      `json:"driver"`
	AgentImage                           string                      `json:"agentImage"`
	AgentFeatures                        map[string]bool             `json:"agentFeatures,omitempty"`
	AuthImage                            string                      `json:"authImage"`
	ComponentStatuses                    []ClusterComponentStatus    `json:"componentStatuses,omitempty"`
	APIEndpoint                          string                      `json:"apiEndpoint,omitempty"`
	ServiceAccountToken                  string                      `json:"serviceAccountToken,omitempty"`
	CACert                               string                      `json:"caCert,omitempty"`
	Capacity                             v1.ResourceList             `json:"capacity,omitempty"`
	Allocatable                          v1.ResourceList             `json:"allocatable,omitempty"`
	AppliedSpec                          ClusterSpec                 `json:"appliedSpec,omitempty"`
	FailedSpec                           *ClusterSpec                `json:"failedSpec,omitempty"`
	Requested                            v1.ResourceList             `json:"requested,omitempty"`
	Limits                               v1.ResourceList             `json:"limits,omitempty"`
	Version                              *version.Info               `json:"version,omitempty"`
	AppliedPodSecurityPolicyTemplateName string                      `json:"appliedPodSecurityPolicyTemplateId"`
	AppliedEnableNetworkPolicy           bool                        `json:"appliedEnableNetworkPolicy" norman:"nocreate,noupdate,default=false"`
	Capabilities                         Capabilities                `json:"capabilities,omitempty"`
	MonitoringStatus                     *MonitoringStatus           `json:"monitoringStatus,omitempty" norman:"nocreate,noupdate"`
	NodeVersion                          int                         `json:"nodeVersion,omitempty"`
	IstioEnabled                         bool                        `json:"istioEnabled,omitempty" norman:"nocreate,noupdate,default=false"`
	CertificatesExpiration               map[string]CertExpiration   `json:"certificatesExpiration,omitempty"`
	ScheduledClusterScanStatus           *ScheduledClusterScanStatus `json:"scheduledClusterScanStatus,omitempty"`
	CurrentCisRunName                    string                      `json:"currentCisRunName,omitempty"`
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

type MapStringInterface map[string]interface{}

func (m *MapStringInterface) DeepCopy() *MapStringInterface {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	dec := gob.NewDecoder(&buf)
	err := enc.Encode(m)
	if err != nil {
		logrus.Errorf("error while deep copying MapStringInterface %v", err)
		return nil
	}

	var copy MapStringInterface
	err = dec.Decode(&copy)
	if err != nil {
		logrus.Errorf("error while deep copying MapStringInterface %v", err)
		return nil
	}

	return &copy
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

func (c *ClusterRegistrationToken) ObjClusterName() string {
	return c.Spec.ObjClusterName()
}

type ClusterRegistrationTokenSpec struct {
	ClusterName string `json:"clusterName" norman:"required,type=reference[cluster]"`
}

func (c *ClusterRegistrationTokenSpec) ObjClusterName() string {
	return c.ClusterName
}

type ClusterRegistrationTokenStatus struct {
	InsecureCommand    string `json:"insecureCommand"`
	Command            string `json:"command"`
	WindowsNodeCommand string `json:"windowsNodeCommand"`
	NodeCommand        string `json:"nodeCommand"`
	ManifestURL        string `json:"manifestUrl"`
	Token              string `json:"token"`
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

func (i *ImportClusterYamlInput) ObjClusterName() string {
	if parts := strings.SplitN(i.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type ImportYamlOutput struct {
	Message string `json:"message,omitempty"`
}

type Capabilities struct {
	LoadBalancerCapabilities LoadBalancerCapabilities `json:"loadBalancerCapabilities,omitempty"`
	IngressCapabilities      []IngressCapabilities    `json:"ingressCapabilities,omitempty"`
	NodePoolScalingSupported bool                     `json:"nodePoolScalingSupported,omitempty"`
	NodePortRange            string                   `json:"nodePortRange,omitempty"`
	TaintSupport             *bool                    `json:"taintSupport,omitempty"`
	PspEnabled               bool                     `json:"pspEnabled,omitempty"`
}

type LoadBalancerCapabilities struct {
	Enabled              *bool    `json:"enabled,omitempty"`
	Provider             string   `json:"provider,omitempty"`
	ProtocolsSupported   []string `json:"protocolsSupported,omitempty"`
	HealthCheckSupported bool     `json:"healthCheckSupported,omitempty"`
}

type IngressCapabilities struct {
	IngressProvider      string `json:"ingressProvider,omitempty"`
	CustomDefaultBackend *bool  `json:"customDefaultBackend,omitempty"`
}

type MonitoringInput struct {
	Version string            `json:"version,omitempty"`
	Answers map[string]string `json:"answers,omitempty"`
}

type MonitoringOutput struct {
	Version string            `json:"version,omitempty"`
	Answers map[string]string `json:"answers,omitempty"`
}

type RestoreFromEtcdBackupInput struct {
	EtcdBackupName   string `json:"etcdBackupName,omitempty" norman:"type=reference[etcdBackup]"`
	RestoreRkeConfig string `json:"restoreRkeConfig,omitempty"`
}

type RotateCertificateInput struct {
	CACertificates bool     `json:"caCertificates,omitempty"`
	Services       []string `json:"services,omitempty" norman:"type=enum,options=etcd|kubelet|kube-apiserver|kube-proxy|kube-scheduler|kube-controller-manager"`
}

type RotateCertificateOutput struct {
	Message string `json:"message,omitempty"`
}

type LocalClusterAuthEndpoint struct {
	Enabled bool   `json:"enabled"`
	FQDN    string `json:"fqdn,omitempty"`
	CACerts string `json:"caCerts,omitempty"`
}

type CertExpiration struct {
	ExpirationDate string `json:"expirationDate,omitempty"`
}

type SaveAsTemplateInput struct {
	ClusterTemplateName         string `json:"clusterTemplateName,omitempty"`
	ClusterTemplateRevisionName string `json:"clusterTemplateRevisionName,omitempty"`
}

type SaveAsTemplateOutput struct {
	ClusterTemplateName         string `json:"clusterTemplateName,omitempty"`
	ClusterTemplateRevisionName string `json:"clusterTemplateRevisionName,omitempty"`
}
