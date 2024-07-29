package client

const (
	ClusterStatusType                                            = "clusterStatus"
	ClusterStatusFieldAADClientCertSecret                        = "aadClientCertSecret"
	ClusterStatusFieldAADClientSecret                            = "aadClientSecret"
	ClusterStatusFieldAKSStatus                                  = "aksStatus"
	ClusterStatusFieldAPIEndpoint                                = "apiEndpoint"
	ClusterStatusFieldAgentFeatures                              = "agentFeatures"
	ClusterStatusFieldAgentImage                                 = "agentImage"
	ClusterStatusFieldAllocatable                                = "allocatable"
	ClusterStatusFieldAppliedAgentEnvVars                        = "appliedAgentEnvVars"
	ClusterStatusFieldAppliedClusterAgentDeploymentCustomization = "appliedClusterAgentDeploymentCustomization"
	ClusterStatusFieldAppliedEnableNetworkPolicy                 = "appliedEnableNetworkPolicy"
	ClusterStatusFieldAppliedSpec                                = "appliedSpec"
	ClusterStatusFieldAuthImage                                  = "authImage"
	ClusterStatusFieldCACert                                     = "caCert"
	ClusterStatusFieldCapabilities                               = "capabilities"
	ClusterStatusFieldCapacity                                   = "capacity"
	ClusterStatusFieldCertificatesExpiration                     = "certificatesExpiration"
	ClusterStatusFieldComponentStatuses                          = "componentStatuses"
	ClusterStatusFieldConditions                                 = "conditions"
	ClusterStatusFieldCurrentCisRunName                          = "currentCisRunName"
	ClusterStatusFieldDriver                                     = "driver"
	ClusterStatusFieldEKSStatus                                  = "eksStatus"
	ClusterStatusFieldFailedSpec                                 = "failedSpec"
	ClusterStatusFieldGKEStatus                                  = "gkeStatus"
	ClusterStatusFieldIstioEnabled                               = "istioEnabled"
	ClusterStatusFieldLimits                                     = "limits"
	ClusterStatusFieldLinuxWorkerCount                           = "linuxWorkerCount"
	ClusterStatusFieldNodeCount                                  = "nodeCount"
	ClusterStatusFieldNodeVersion                                = "nodeVersion"
	ClusterStatusFieldOpenStackSecret                            = "openStackSecret"
	ClusterStatusFieldPrivateRegistrySecret                      = "privateRegistrySecret"
	ClusterStatusFieldProvider                                   = "provider"
	ClusterStatusFieldRequested                                  = "requested"
	ClusterStatusFieldS3CredentialSecret                         = "s3CredentialSecret"
	ClusterStatusFieldServiceAccountTokenSecret                  = "serviceAccountTokenSecret"
	ClusterStatusFieldVersion                                    = "version"
	ClusterStatusFieldVirtualCenterSecret                        = "virtualCenterSecret"
	ClusterStatusFieldVsphereSecret                              = "vsphereSecret"
	ClusterStatusFieldWeavePasswordSecret                        = "weavePasswordSecret"
	ClusterStatusFieldWindowsWorkerCount                         = "windowsWorkerCount"
)

type ClusterStatus struct {
	AADClientCertSecret                        string                        `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret                            string                        `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	AKSStatus                                  *AKSStatus                    `json:"aksStatus,omitempty" yaml:"aksStatus,omitempty"`
	APIEndpoint                                string                        `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`
	AgentFeatures                              map[string]bool               `json:"agentFeatures,omitempty" yaml:"agentFeatures,omitempty"`
	AgentImage                                 string                        `json:"agentImage,omitempty" yaml:"agentImage,omitempty"`
	Allocatable                                map[string]string             `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	AppliedAgentEnvVars                        []EnvVar                      `json:"appliedAgentEnvVars,omitempty" yaml:"appliedAgentEnvVars,omitempty"`
	AppliedClusterAgentDeploymentCustomization *AgentDeploymentCustomization `json:"appliedClusterAgentDeploymentCustomization,omitempty" yaml:"appliedClusterAgentDeploymentCustomization,omitempty"`
	AppliedEnableNetworkPolicy                 bool                          `json:"appliedEnableNetworkPolicy,omitempty" yaml:"appliedEnableNetworkPolicy,omitempty"`
	AppliedSpec                                *ClusterSpec                  `json:"appliedSpec,omitempty" yaml:"appliedSpec,omitempty"`
	AuthImage                                  string                        `json:"authImage,omitempty" yaml:"authImage,omitempty"`
	CACert                                     string                        `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Capabilities                               *Capabilities                 `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Capacity                                   map[string]string             `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	CertificatesExpiration                     map[string]CertExpiration     `json:"certificatesExpiration,omitempty" yaml:"certificatesExpiration,omitempty"`
	ComponentStatuses                          []ClusterComponentStatus      `json:"componentStatuses,omitempty" yaml:"componentStatuses,omitempty"`
	Conditions                                 []ClusterCondition            `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	CurrentCisRunName                          string                        `json:"currentCisRunName,omitempty" yaml:"currentCisRunName,omitempty"`
	Driver                                     string                        `json:"driver,omitempty" yaml:"driver,omitempty"`
	EKSStatus                                  *EKSStatus                    `json:"eksStatus,omitempty" yaml:"eksStatus,omitempty"`
	FailedSpec                                 *ClusterSpec                  `json:"failedSpec,omitempty" yaml:"failedSpec,omitempty"`
	GKEStatus                                  *GKEStatus                    `json:"gkeStatus,omitempty" yaml:"gkeStatus,omitempty"`
	IstioEnabled                               bool                          `json:"istioEnabled,omitempty" yaml:"istioEnabled,omitempty"`
	Limits                                     map[string]string             `json:"limits,omitempty" yaml:"limits,omitempty"`
	LinuxWorkerCount                           int64                         `json:"linuxWorkerCount,omitempty" yaml:"linuxWorkerCount,omitempty"`
	NodeCount                                  int64                         `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	NodeVersion                                int64                         `json:"nodeVersion,omitempty" yaml:"nodeVersion,omitempty"`
	OpenStackSecret                            string                        `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	PrivateRegistrySecret                      string                        `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	Provider                                   string                        `json:"provider,omitempty" yaml:"provider,omitempty"`
	Requested                                  map[string]string             `json:"requested,omitempty" yaml:"requested,omitempty"`
	S3CredentialSecret                         string                        `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	ServiceAccountTokenSecret                  string                        `json:"serviceAccountTokenSecret,omitempty" yaml:"serviceAccountTokenSecret,omitempty"`
	Version                                    *Info                         `json:"version,omitempty" yaml:"version,omitempty"`
	VirtualCenterSecret                        string                        `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret                              string                        `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret                        string                        `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
	WindowsWorkerCount                         int64                         `json:"windowsWorkerCount,omitempty" yaml:"windowsWorkerCount,omitempty"`
}
