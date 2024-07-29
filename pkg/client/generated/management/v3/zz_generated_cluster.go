package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterType                                                      = "cluster"
	ClusterFieldAADClientCertSecret                                  = "aadClientCertSecret"
	ClusterFieldAADClientSecret                                      = "aadClientSecret"
	ClusterFieldAKSConfig                                            = "aksConfig"
	ClusterFieldAKSStatus                                            = "aksStatus"
	ClusterFieldAPIEndpoint                                          = "apiEndpoint"
	ClusterFieldAgentEnvVars                                         = "agentEnvVars"
	ClusterFieldAgentFeatures                                        = "agentFeatures"
	ClusterFieldAgentImage                                           = "agentImage"
	ClusterFieldAgentImageOverride                                   = "agentImageOverride"
	ClusterFieldAllocatable                                          = "allocatable"
	ClusterFieldAnnotations                                          = "annotations"
	ClusterFieldAppliedAgentEnvVars                                  = "appliedAgentEnvVars"
	ClusterFieldAppliedClusterAgentDeploymentCustomization           = "appliedClusterAgentDeploymentCustomization"
	ClusterFieldAppliedEnableNetworkPolicy                           = "appliedEnableNetworkPolicy"
	ClusterFieldAppliedSpec                                          = "appliedSpec"
	ClusterFieldAuthImage                                            = "authImage"
	ClusterFieldCACert                                               = "caCert"
	ClusterFieldCapabilities                                         = "capabilities"
	ClusterFieldCapacity                                             = "capacity"
	ClusterFieldCertificatesExpiration                               = "certificatesExpiration"
	ClusterFieldClusterAgentDeploymentCustomization                  = "clusterAgentDeploymentCustomization"
	ClusterFieldClusterSecrets                                       = "clusterSecrets"
	ClusterFieldClusterTemplateAnswers                               = "answers"
	ClusterFieldClusterTemplateID                                    = "clusterTemplateId"
	ClusterFieldClusterTemplateQuestions                             = "questions"
	ClusterFieldClusterTemplateRevisionID                            = "clusterTemplateRevisionId"
	ClusterFieldComponentStatuses                                    = "componentStatuses"
	ClusterFieldConditions                                           = "conditions"
	ClusterFieldCreated                                              = "created"
	ClusterFieldCreatorID                                            = "creatorId"
	ClusterFieldCurrentCisRunName                                    = "currentCisRunName"
	ClusterFieldDefaultClusterRoleForProjectMembers                  = "defaultClusterRoleForProjectMembers"
	ClusterFieldDefaultPodSecurityAdmissionConfigurationTemplateName = "defaultPodSecurityAdmissionConfigurationTemplateName"
	ClusterFieldDescription                                          = "description"
	ClusterFieldDesiredAgentImage                                    = "desiredAgentImage"
	ClusterFieldDesiredAuthImage                                     = "desiredAuthImage"
	ClusterFieldDockerRootDir                                        = "dockerRootDir"
	ClusterFieldDriver                                               = "driver"
	ClusterFieldEKSConfig                                            = "eksConfig"
	ClusterFieldEKSStatus                                            = "eksStatus"
	ClusterFieldEnableNetworkPolicy                                  = "enableNetworkPolicy"
	ClusterFieldFailedSpec                                           = "failedSpec"
	ClusterFieldFleetAgentDeploymentCustomization                    = "fleetAgentDeploymentCustomization"
	ClusterFieldFleetWorkspaceName                                   = "fleetWorkspaceName"
	ClusterFieldGKEConfig                                            = "gkeConfig"
	ClusterFieldGKEStatus                                            = "gkeStatus"
	ClusterFieldImportedConfig                                       = "importedConfig"
	ClusterFieldInternal                                             = "internal"
	ClusterFieldIstioEnabled                                         = "istioEnabled"
	ClusterFieldK3sConfig                                            = "k3sConfig"
	ClusterFieldLabels                                               = "labels"
	ClusterFieldLimits                                               = "limits"
	ClusterFieldLinuxWorkerCount                                     = "linuxWorkerCount"
	ClusterFieldLocalClusterAuthEndpoint                             = "localClusterAuthEndpoint"
	ClusterFieldName                                                 = "name"
	ClusterFieldNodeCount                                            = "nodeCount"
	ClusterFieldNodeVersion                                          = "nodeVersion"
	ClusterFieldOpenStackSecret                                      = "openStackSecret"
	ClusterFieldOwnerReferences                                      = "ownerReferences"
	ClusterFieldPrivateRegistrySecret                                = "privateRegistrySecret"
	ClusterFieldProvider                                             = "provider"
	ClusterFieldRancherKubernetesEngineConfig                        = "rancherKubernetesEngineConfig"
	ClusterFieldRemoved                                              = "removed"
	ClusterFieldRequested                                            = "requested"
	ClusterFieldRke2Config                                           = "rke2Config"
	ClusterFieldS3CredentialSecret                                   = "s3CredentialSecret"
	ClusterFieldServiceAccountTokenSecret                            = "serviceAccountTokenSecret"
	ClusterFieldState                                                = "state"
	ClusterFieldTransitioning                                        = "transitioning"
	ClusterFieldTransitioningMessage                                 = "transitioningMessage"
	ClusterFieldUUID                                                 = "uuid"
	ClusterFieldVersion                                              = "version"
	ClusterFieldVirtualCenterSecret                                  = "virtualCenterSecret"
	ClusterFieldVsphereSecret                                        = "vsphereSecret"
	ClusterFieldWeavePasswordSecret                                  = "weavePasswordSecret"
	ClusterFieldWindowsPreferedCluster                               = "windowsPreferedCluster"
	ClusterFieldWindowsWorkerCount                                   = "windowsWorkerCount"
)

type Cluster struct {
	types.Resource
	AADClientCertSecret                                  string                         `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret                                      string                         `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	AKSConfig                                            *AKSClusterConfigSpec          `json:"aksConfig,omitempty" yaml:"aksConfig,omitempty"`
	AKSStatus                                            *AKSStatus                     `json:"aksStatus,omitempty" yaml:"aksStatus,omitempty"`
	APIEndpoint                                          string                         `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`
	AgentEnvVars                                         []EnvVar                       `json:"agentEnvVars,omitempty" yaml:"agentEnvVars,omitempty"`
	AgentFeatures                                        map[string]bool                `json:"agentFeatures,omitempty" yaml:"agentFeatures,omitempty"`
	AgentImage                                           string                         `json:"agentImage,omitempty" yaml:"agentImage,omitempty"`
	AgentImageOverride                                   string                         `json:"agentImageOverride,omitempty" yaml:"agentImageOverride,omitempty"`
	Allocatable                                          map[string]string              `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	Annotations                                          map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AppliedAgentEnvVars                                  []EnvVar                       `json:"appliedAgentEnvVars,omitempty" yaml:"appliedAgentEnvVars,omitempty"`
	AppliedClusterAgentDeploymentCustomization           *AgentDeploymentCustomization  `json:"appliedClusterAgentDeploymentCustomization,omitempty" yaml:"appliedClusterAgentDeploymentCustomization,omitempty"`
	AppliedEnableNetworkPolicy                           bool                           `json:"appliedEnableNetworkPolicy,omitempty" yaml:"appliedEnableNetworkPolicy,omitempty"`
	AppliedSpec                                          *ClusterSpec                   `json:"appliedSpec,omitempty" yaml:"appliedSpec,omitempty"`
	AuthImage                                            string                         `json:"authImage,omitempty" yaml:"authImage,omitempty"`
	CACert                                               string                         `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Capabilities                                         *Capabilities                  `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Capacity                                             map[string]string              `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	CertificatesExpiration                               map[string]CertExpiration      `json:"certificatesExpiration,omitempty" yaml:"certificatesExpiration,omitempty"`
	ClusterAgentDeploymentCustomization                  *AgentDeploymentCustomization  `json:"clusterAgentDeploymentCustomization,omitempty" yaml:"clusterAgentDeploymentCustomization,omitempty"`
	ClusterSecrets                                       *ClusterSecrets                `json:"clusterSecrets,omitempty" yaml:"clusterSecrets,omitempty"`
	ClusterTemplateAnswers                               *Answer                        `json:"answers,omitempty" yaml:"answers,omitempty"`
	ClusterTemplateID                                    string                         `json:"clusterTemplateId,omitempty" yaml:"clusterTemplateId,omitempty"`
	ClusterTemplateQuestions                             []Question                     `json:"questions,omitempty" yaml:"questions,omitempty"`
	ClusterTemplateRevisionID                            string                         `json:"clusterTemplateRevisionId,omitempty" yaml:"clusterTemplateRevisionId,omitempty"`
	ComponentStatuses                                    []ClusterComponentStatus       `json:"componentStatuses,omitempty" yaml:"componentStatuses,omitempty"`
	Conditions                                           []ClusterCondition             `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created                                              string                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                                            string                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	CurrentCisRunName                                    string                         `json:"currentCisRunName,omitempty" yaml:"currentCisRunName,omitempty"`
	DefaultClusterRoleForProjectMembers                  string                         `json:"defaultClusterRoleForProjectMembers,omitempty" yaml:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityAdmissionConfigurationTemplateName string                         `json:"defaultPodSecurityAdmissionConfigurationTemplateName,omitempty" yaml:"defaultPodSecurityAdmissionConfigurationTemplateName,omitempty"`
	Description                                          string                         `json:"description,omitempty" yaml:"description,omitempty"`
	DesiredAgentImage                                    string                         `json:"desiredAgentImage,omitempty" yaml:"desiredAgentImage,omitempty"`
	DesiredAuthImage                                     string                         `json:"desiredAuthImage,omitempty" yaml:"desiredAuthImage,omitempty"`
	DockerRootDir                                        string                         `json:"dockerRootDir,omitempty" yaml:"dockerRootDir,omitempty"`
	Driver                                               string                         `json:"driver,omitempty" yaml:"driver,omitempty"`
	EKSConfig                                            *EKSClusterConfigSpec          `json:"eksConfig,omitempty" yaml:"eksConfig,omitempty"`
	EKSStatus                                            *EKSStatus                     `json:"eksStatus,omitempty" yaml:"eksStatus,omitempty"`
	EnableNetworkPolicy                                  *bool                          `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	FailedSpec                                           *ClusterSpec                   `json:"failedSpec,omitempty" yaml:"failedSpec,omitempty"`
	FleetAgentDeploymentCustomization                    *AgentDeploymentCustomization  `json:"fleetAgentDeploymentCustomization,omitempty" yaml:"fleetAgentDeploymentCustomization,omitempty"`
	FleetWorkspaceName                                   string                         `json:"fleetWorkspaceName,omitempty" yaml:"fleetWorkspaceName,omitempty"`
	GKEConfig                                            *GKEClusterConfigSpec          `json:"gkeConfig,omitempty" yaml:"gkeConfig,omitempty"`
	GKEStatus                                            *GKEStatus                     `json:"gkeStatus,omitempty" yaml:"gkeStatus,omitempty"`
	ImportedConfig                                       *ImportedConfig                `json:"importedConfig,omitempty" yaml:"importedConfig,omitempty"`
	Internal                                             bool                           `json:"internal,omitempty" yaml:"internal,omitempty"`
	IstioEnabled                                         bool                           `json:"istioEnabled,omitempty" yaml:"istioEnabled,omitempty"`
	K3sConfig                                            *K3sConfig                     `json:"k3sConfig,omitempty" yaml:"k3sConfig,omitempty"`
	Labels                                               map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	Limits                                               map[string]string              `json:"limits,omitempty" yaml:"limits,omitempty"`
	LinuxWorkerCount                                     int64                          `json:"linuxWorkerCount,omitempty" yaml:"linuxWorkerCount,omitempty"`
	LocalClusterAuthEndpoint                             *LocalClusterAuthEndpoint      `json:"localClusterAuthEndpoint,omitempty" yaml:"localClusterAuthEndpoint,omitempty"`
	Name                                                 string                         `json:"name,omitempty" yaml:"name,omitempty"`
	NodeCount                                            int64                          `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	NodeVersion                                          int64                          `json:"nodeVersion,omitempty" yaml:"nodeVersion,omitempty"`
	OpenStackSecret                                      string                         `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	OwnerReferences                                      []OwnerReference               `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PrivateRegistrySecret                                string                         `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	Provider                                             string                         `json:"provider,omitempty" yaml:"provider,omitempty"`
	RancherKubernetesEngineConfig                        *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty" yaml:"rancherKubernetesEngineConfig,omitempty"`
	Removed                                              string                         `json:"removed,omitempty" yaml:"removed,omitempty"`
	Requested                                            map[string]string              `json:"requested,omitempty" yaml:"requested,omitempty"`
	Rke2Config                                           *Rke2Config                    `json:"rke2Config,omitempty" yaml:"rke2Config,omitempty"`
	S3CredentialSecret                                   string                         `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	ServiceAccountTokenSecret                            string                         `json:"serviceAccountTokenSecret,omitempty" yaml:"serviceAccountTokenSecret,omitempty"`
	State                                                string                         `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning                                        string                         `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage                                 string                         `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                                                 string                         `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version                                              *Info                          `json:"version,omitempty" yaml:"version,omitempty"`
	VirtualCenterSecret                                  string                         `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret                                        string                         `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret                                  string                         `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
	WindowsPreferedCluster                               bool                           `json:"windowsPreferedCluster,omitempty" yaml:"windowsPreferedCluster,omitempty"`
	WindowsWorkerCount                                   int64                          `json:"windowsWorkerCount,omitempty" yaml:"windowsWorkerCount,omitempty"`
}

type ClusterCollection struct {
	types.Collection
	Data   []Cluster `json:"data,omitempty"`
	client *ClusterClient
}

type ClusterClient struct {
	apiClient *Client
}

type ClusterOperations interface {
	List(opts *types.ListOpts) (*ClusterCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterCollection, error)
	Create(opts *Cluster) (*Cluster, error)
	Update(existing *Cluster, updates interface{}) (*Cluster, error)
	Replace(existing *Cluster) (*Cluster, error)
	ByID(id string) (*Cluster, error)
	Delete(container *Cluster) error

	ActionBackupEtcd(resource *Cluster) error

	ActionExportYaml(resource *Cluster) (*ExportOutput, error)

	ActionGenerateKubeconfig(resource *Cluster) (*GenerateKubeConfigOutput, error)

	ActionImportYaml(resource *Cluster, input *ImportClusterYamlInput) (*ImportYamlOutput, error)

	ActionRestoreFromEtcdBackup(resource *Cluster, input *RestoreFromEtcdBackupInput) error

	ActionRotateCertificates(resource *Cluster, input *RotateCertificateInput) (*RotateCertificateOutput, error)

	ActionRotateEncryptionKey(resource *Cluster) (*RotateEncryptionKeyOutput, error)

	ActionSaveAsTemplate(resource *Cluster, input *SaveAsTemplateInput) (*SaveAsTemplateOutput, error)
}

func newClusterClient(apiClient *Client) *ClusterClient {
	return &ClusterClient{
		apiClient: apiClient,
	}
}

func (c *ClusterClient) Create(container *Cluster) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoCreate(ClusterType, container, resp)
	return resp, err
}

func (c *ClusterClient) Update(existing *Cluster, updates interface{}) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoUpdate(ClusterType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterClient) Replace(obj *Cluster) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoReplace(ClusterType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterClient) List(opts *types.ListOpts) (*ClusterCollection, error) {
	resp := &ClusterCollection{}
	err := c.apiClient.Ops.DoList(ClusterType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterClient) ListAll(opts *types.ListOpts) (*ClusterCollection, error) {
	resp := &ClusterCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (cc *ClusterCollection) Next() (*ClusterCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterClient) ByID(id string) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoByID(ClusterType, id, resp)
	return resp, err
}

func (c *ClusterClient) Delete(container *Cluster) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterType, &container.Resource)
}

func (c *ClusterClient) ActionBackupEtcd(resource *Cluster) error {
	err := c.apiClient.Ops.DoAction(ClusterType, "backupEtcd", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterClient) ActionExportYaml(resource *Cluster) (*ExportOutput, error) {
	resp := &ExportOutput{}
	err := c.apiClient.Ops.DoAction(ClusterType, "exportYaml", &resource.Resource, nil, resp)
	return resp, err
}

func (c *ClusterClient) ActionGenerateKubeconfig(resource *Cluster) (*GenerateKubeConfigOutput, error) {
	resp := &GenerateKubeConfigOutput{}
	err := c.apiClient.Ops.DoAction(ClusterType, "generateKubeconfig", &resource.Resource, nil, resp)
	return resp, err
}

func (c *ClusterClient) ActionImportYaml(resource *Cluster, input *ImportClusterYamlInput) (*ImportYamlOutput, error) {
	resp := &ImportYamlOutput{}
	err := c.apiClient.Ops.DoAction(ClusterType, "importYaml", &resource.Resource, input, resp)
	return resp, err
}

func (c *ClusterClient) ActionRestoreFromEtcdBackup(resource *Cluster, input *RestoreFromEtcdBackupInput) error {
	err := c.apiClient.Ops.DoAction(ClusterType, "restoreFromEtcdBackup", &resource.Resource, input, nil)
	return err
}

func (c *ClusterClient) ActionRotateCertificates(resource *Cluster, input *RotateCertificateInput) (*RotateCertificateOutput, error) {
	resp := &RotateCertificateOutput{}
	err := c.apiClient.Ops.DoAction(ClusterType, "rotateCertificates", &resource.Resource, input, resp)
	return resp, err
}

func (c *ClusterClient) ActionRotateEncryptionKey(resource *Cluster) (*RotateEncryptionKeyOutput, error) {
	resp := &RotateEncryptionKeyOutput{}
	err := c.apiClient.Ops.DoAction(ClusterType, "rotateEncryptionKey", &resource.Resource, nil, resp)
	return resp, err
}

func (c *ClusterClient) ActionSaveAsTemplate(resource *Cluster, input *SaveAsTemplateInput) (*SaveAsTemplateOutput, error) {
	resp := &SaveAsTemplateOutput{}
	err := c.apiClient.Ops.DoAction(ClusterType, "saveAsTemplate", &resource.Resource, input, resp)
	return resp, err
}
