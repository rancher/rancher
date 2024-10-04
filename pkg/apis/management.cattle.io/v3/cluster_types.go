package v3

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"

	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	rketypes "github.com/rancher/rke/types"
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
	ClusterActionBackupEtcd            = "backupEtcd"
	ClusterActionRestoreFromEtcdBackup = "restoreFromEtcdBackup"
	ClusterActionRotateCertificates    = "rotateCertificates"
	ClusterActionRotateEncryptionKey   = "rotateEncryptionKey"
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
	// ClusterConditionDefaultProjectCreated true when default project has been created
	ClusterConditionDefaultProjectCreated condition.Cond = "DefaultProjectCreated"
	// ClusterConditionSystemProjectCreated true when system project has been created
	ClusterConditionSystemProjectCreated condition.Cond = "SystemProjectCreated"
	// Deprecated: ClusterConditionDefaultNamespaceAssigned true when cluster's default namespace has been initially assigned
	ClusterConditionDefaultNamespaceAssigned condition.Cond = "DefaultNamespaceAssigned"
	// Deprecated: ClusterConditionSystemNamespacesAssigned true when cluster's system namespaces has been initially assigned to
	// a system project
	ClusterConditionSystemNamespacesAssigned             condition.Cond = "SystemNamespacesAssigned"
	ClusterConditionAddonDeploy                          condition.Cond = "AddonDeploy"
	ClusterConditionSystemAccountCreated                 condition.Cond = "SystemAccountCreated"
	ClusterConditionAgentDeployed                        condition.Cond = "AgentDeployed"
	ClusterConditionGlobalAdminsSynced                   condition.Cond = "GlobalAdminsSynced"
	ClusterConditionInitialRolesPopulated                condition.Cond = "InitialRolesPopulated"
	ClusterConditionServiceAccountMigrated               condition.Cond = "ServiceAccountMigrated"
	ClusterConditionAlertingEnabled                      condition.Cond = "AlertingEnabled"
	ClusterConditionSecretsMigrated                      condition.Cond = "SecretsMigrated"
	ClusterConditionServiceAccountSecretsMigrated        condition.Cond = "ServiceAccountSecretsMigrated"
	ClusterConditionHarvesterCloudProviderConfigMigrated condition.Cond = "HarvesterCloudProviderConfigMigrated"
	ClusterConditionACISecretsMigrated                   condition.Cond = "ACISecretsMigrated"
	ClusterConditionRKESecretsMigrated                   condition.Cond = "RKESecretsMigrated"

	ClusterDriverImported = "imported"
	ClusterDriverLocal    = "local"
	ClusterDriverRKE      = "rancherKubernetesEngine"
	ClusterDriverK3s      = "k3s"
	ClusterDriverK3os     = "k3os"
	ClusterDriverRke2     = "rke2"
	ClusterDriverAKS      = "AKS"
	ClusterDriverEKS      = "EKS"
	ClusterDriverGKE      = "GKE"
	ClusterDriverRancherD = "rancherd"

	ClusterPrivateRegistrySecret = "PrivateRegistrySecret"
	ClusterPrivateRegistryURL    = "PrivateRegistryURL"
)

// +genclient
// +kubebuilder:skipversion
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
	DesiredAgentImage                                    string                                  `json:"desiredAgentImage"`
	DesiredAuthImage                                     string                                  `json:"desiredAuthImage"`
	AgentImageOverride                                   string                                  `json:"agentImageOverride"`
	AgentEnvVars                                         []v1.EnvVar                             `json:"agentEnvVars,omitempty"`
	RancherKubernetesEngineConfig                        *rketypes.RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
	DefaultPodSecurityAdmissionConfigurationTemplateName string                                  `json:"defaultPodSecurityAdmissionConfigurationTemplateName,omitempty"`
	DefaultClusterRoleForProjectMembers                  string                                  `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
	DockerRootDir                                        string                                  `json:"dockerRootDir,omitempty" norman:"default=/var/lib/docker"`
	EnableNetworkPolicy                                  *bool                                   `json:"enableNetworkPolicy" norman:"default=false"`
	WindowsPreferedCluster                               bool                                    `json:"windowsPreferedCluster" norman:"noupdate"`
	LocalClusterAuthEndpoint                             LocalClusterAuthEndpoint                `json:"localClusterAuthEndpoint,omitempty"`
	ClusterSecrets                                       ClusterSecrets                          `json:"clusterSecrets" norman:"nocreate,noupdate"`
	ClusterAgentDeploymentCustomization                  *AgentDeploymentCustomization           `json:"clusterAgentDeploymentCustomization,omitempty"`
	FleetAgentDeploymentCustomization                    *AgentDeploymentCustomization           `json:"fleetAgentDeploymentCustomization,omitempty"`
}

type AgentDeploymentCustomization struct {
	AppendTolerations            []v1.Toleration          `json:"appendTolerations,omitempty"`
	OverrideAffinity             *v1.Affinity             `json:"overrideAffinity,omitempty"`
	OverrideResourceRequirements *v1.ResourceRequirements `json:"overrideResourceRequirements,omitempty"`
}

type ClusterSpec struct {
	ClusterSpecBase
	DisplayName                         string                      `json:"displayName" norman:"required"`
	Description                         string                      `json:"description"`
	Internal                            bool                        `json:"internal" norman:"nocreate,noupdate"`
	K3sConfig                           *K3sConfig                  `json:"k3sConfig,omitempty"`
	Rke2Config                          *Rke2Config                 `json:"rke2Config,omitempty"`
	ImportedConfig                      *ImportedConfig             `json:"importedConfig,omitempty" norman:"nocreate,noupdate"`
	GoogleKubernetesEngineConfig        *MapStringInterface         `json:"googleKubernetesEngineConfig,omitempty"`
	AzureKubernetesServiceConfig        *MapStringInterface         `json:"azureKubernetesServiceConfig,omitempty"`
	AmazonElasticContainerServiceConfig *MapStringInterface         `json:"amazonElasticContainerServiceConfig,omitempty"`
	GenericEngineConfig                 *MapStringInterface         `json:"genericEngineConfig,omitempty"`
	AKSConfig                           *aksv1.AKSClusterConfigSpec `json:"aksConfig,omitempty"`
	EKSConfig                           *eksv1.EKSClusterConfigSpec `json:"eksConfig,omitempty"`
	GKEConfig                           *gkev1.GKEClusterConfigSpec `json:"gkeConfig,omitempty"`
	ClusterTemplateName                 string                      `json:"clusterTemplateName,omitempty" norman:"type=reference[clusterTemplate],nocreate,noupdate"`
	ClusterTemplateRevisionName         string                      `json:"clusterTemplateRevisionName,omitempty" norman:"type=reference[clusterTemplateRevision]"`
	ClusterTemplateAnswers              Answer                      `json:"answers,omitempty"`
	ClusterTemplateQuestions            []Question                  `json:"questions,omitempty" norman:"nocreate,noupdate"`
	FleetWorkspaceName                  string                      `json:"fleetWorkspaceName,omitempty"`
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
	Driver                     string                    `json:"driver"`
	Provider                   string                    `json:"provider"`
	AgentImage                 string                    `json:"agentImage"`
	AppliedAgentEnvVars        []v1.EnvVar               `json:"appliedAgentEnvVars,omitempty"`
	AgentFeatures              map[string]bool           `json:"agentFeatures,omitempty"`
	AuthImage                  string                    `json:"authImage"`
	ComponentStatuses          []ClusterComponentStatus  `json:"componentStatuses,omitempty"`
	APIEndpoint                string                    `json:"apiEndpoint,omitempty"`
	ServiceAccountToken        string                    `json:"serviceAccountToken,omitempty"`
	ServiceAccountTokenSecret  string                    `json:"serviceAccountTokenSecret,omitempty"`
	CACert                     string                    `json:"caCert,omitempty"`
	Capacity                   v1.ResourceList           `json:"capacity,omitempty"`
	Allocatable                v1.ResourceList           `json:"allocatable,omitempty"`
	AppliedSpec                ClusterSpec               `json:"appliedSpec,omitempty"`
	FailedSpec                 *ClusterSpec              `json:"failedSpec,omitempty"`
	Requested                  v1.ResourceList           `json:"requested,omitempty"`
	Limits                     v1.ResourceList           `json:"limits,omitempty"`
	Version                    *version.Info             `json:"version,omitempty"`
	AppliedEnableNetworkPolicy bool                      `json:"appliedEnableNetworkPolicy" norman:"nocreate,noupdate,default=false"`
	Capabilities               Capabilities              `json:"capabilities,omitempty"`
	NodeVersion                int                       `json:"nodeVersion,omitempty"`
	NodeCount                  int                       `json:"nodeCount,omitempty" norman:"nocreate,noupdate"`
	LinuxWorkerCount           int                       `json:"linuxWorkerCount,omitempty" norman:"nocreate,noupdate"`
	WindowsWorkerCount         int                       `json:"windowsWorkerCount,omitempty" norman:"nocreate,noupdate"`
	IstioEnabled               bool                      `json:"istioEnabled,omitempty" norman:"nocreate,noupdate,default=false"`
	CertificatesExpiration     map[string]CertExpiration `json:"certificatesExpiration,omitempty"`
	CurrentCisRunName          string                    `json:"currentCisRunName,omitempty"`
	AKSStatus                  AKSStatus                 `json:"aksStatus,omitempty" norman:"nocreate,noupdate"`
	EKSStatus                  EKSStatus                 `json:"eksStatus,omitempty" norman:"nocreate,noupdate"`
	GKEStatus                  GKEStatus                 `json:"gkeStatus,omitempty" norman:"nocreate,noupdate"`
	PrivateRegistrySecret      string                    `json:"privateRegistrySecret,omitempty" norman:"nocreate,noupdate"` // Deprecated: use ClusterSpec.ClusterSecrets.PrivateRegistrySecret instead
	S3CredentialSecret         string                    `json:"s3CredentialSecret,omitempty" norman:"nocreate,noupdate"`    // Deprecated: use ClusterSpec.ClusterSecrets.S3CredentialSecret instead
	WeavePasswordSecret        string                    `json:"weavePasswordSecret,omitempty" norman:"nocreate,noupdate"`   // Deprecated: use ClusterSpec.ClusterSecrets.WeavePasswordSecret instead
	VsphereSecret              string                    `json:"vsphereSecret,omitempty" norman:"nocreate,noupdate"`         // Deprecated: use ClusterSpec.ClusterSecrets.VsphereSecret instead
	VirtualCenterSecret        string                    `json:"virtualCenterSecret,omitempty" norman:"nocreate,noupdate"`   // Deprecated: use ClusterSpec.ClusterSecrets.VirtualCenterSecret instead
	OpenStackSecret            string                    `json:"openStackSecret,omitempty" norman:"nocreate,noupdate"`       // Deprecated: use ClusterSpec.ClusterSecrets.OpenStackSecret instead
	AADClientSecret            string                    `json:"aadClientSecret,omitempty" norman:"nocreate,noupdate"`       // Deprecated: use ClusterSpec.ClusterSecrets.AADClientSecret instead
	AADClientCertSecret        string                    `json:"aadClientCertSecret,omitempty" norman:"nocreate,noupdate"`   // Deprecated: use ClusterSpec.ClusterSecrets.AADClientCertSecret instead

	AppliedClusterAgentDeploymentCustomization *AgentDeploymentCustomization `json:"appliedClusterAgentDeploymentCustomization,omitempty"`
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

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
	InsecureCommand            string `json:"insecureCommand"`
	Command                    string `json:"command"`
	WindowsNodeCommand         string `json:"windowsNodeCommand"`
	InsecureWindowsNodeCommand string `json:"insecureWindowsNodeCommand"`
	NodeCommand                string `json:"nodeCommand"`
	InsecureNodeCommand        string `json:"insecureNodeCommand"`
	ManifestURL                string `json:"manifestUrl"`
	Token                      string `json:"token"`
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

type RotateEncryptionKeyOutput struct {
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

type AKSStatus struct {
	UpstreamSpec          *aksv1.AKSClusterConfigSpec `json:"upstreamSpec"`
	PrivateRequiresTunnel *bool                       `json:"privateRequiresTunnel"`
	RBACEnabled           *bool                       `json:"rbacEnabled"`
}

type EKSStatus struct {
	UpstreamSpec                  *eksv1.EKSClusterConfigSpec `json:"upstreamSpec"`
	VirtualNetwork                string                      `json:"virtualNetwork"`
	Subnets                       []string                    `json:"subnets"`
	SecurityGroups                []string                    `json:"securityGroups"`
	PrivateRequiresTunnel         *bool                       `json:"privateRequiresTunnel"`
	ManagedLaunchTemplateID       string                      `json:"managedLaunchTemplateID"`
	ManagedLaunchTemplateVersions map[string]string           `json:"managedLaunchTemplateVersions"`
	GeneratedNodeRole             string                      `json:"generatedNodeRole"`
}

type GKEStatus struct {
	UpstreamSpec          *gkev1.GKEClusterConfigSpec `json:"upstreamSpec"`
	PrivateRequiresTunnel *bool                       `json:"privateRequiresTunnel"`
}

type ClusterSecrets struct {
	PrivateRegistrySecret            string `json:"privateRegistrySecret,omitempty" norman:"nocreate,noupdate"`
	PrivateRegistryURL               string `json:"privateRegistryURL,omitempty" norman:"nocreate,noupdate"`
	S3CredentialSecret               string `json:"s3CredentialSecret,omitempty" norman:"nocreate,noupdate"`
	WeavePasswordSecret              string `json:"weavePasswordSecret,omitempty" norman:"nocreate,noupdate"`
	VsphereSecret                    string `json:"vsphereSecret,omitempty" norman:"nocreate,noupdate"`
	VirtualCenterSecret              string `json:"virtualCenterSecret,omitempty" norman:"nocreate,noupdate"`
	OpenStackSecret                  string `json:"openStackSecret,omitempty" norman:"nocreate,noupdate"`
	AADClientSecret                  string `json:"aadClientSecret,omitempty" norman:"nocreate,noupdate"`
	AADClientCertSecret              string `json:"aadClientCertSecret,omitempty" norman:"nocreate,noupdate"`
	ACIAPICUserKeySecret             string `json:"aciAPICUserKeySecret,omitempty" norman:"nocreate,noupdate"`
	ACITokenSecret                   string `json:"aciTokenSecret,omitempty" norman:"nocreate,noupdate"`
	ACIKafkaClientKeySecret          string `json:"aciKafkaClientKeySecret,omitempty" norman:"nocreate,noupdate"`
	SecretsEncryptionProvidersSecret string `json:"secretsEncryptionProvidersSecret,omitempty" norman:"nocreate,noupdate"`
	BastionHostSSHKeySecret          string `json:"bastionHostSSHKeySecret,omitempty" norman:"nocreate,noupdate"`
	KubeletExtraEnvSecret            string `json:"kubeletExtraEnvSecret,omitempty" norman:"nocreate,noupdate"`
	PrivateRegistryECRSecret         string `json:"privateRegistryECRSecret,omitempty" norman:"nocreate,noupdate"`
}

// GetSecret gets a reference to a secret by its field name, either from the ClusterSecrets field or the Status field.
// Spec.ClusterSecrets.* is preferred because the secret fields on Status are deprecated.
func (c *Cluster) GetSecret(key string) string {
	clusterSecrets := reflect.ValueOf(&c.Spec.ClusterSecrets).Elem()
	secret := clusterSecrets.FieldByName(key)
	if secret.IsValid() && secret.String() != "" {
		return secret.String()
	}
	status := reflect.ValueOf(&c.Status).Elem()
	secret = status.FieldByName(key)
	if secret.IsValid() {
		return secret.String()
	}
	return ""
}
