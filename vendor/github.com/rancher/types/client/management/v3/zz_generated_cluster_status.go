package client

const (
	ClusterStatusType                                      = "clusterStatus"
	ClusterStatusFieldAPIEndpoint                          = "apiEndpoint"
	ClusterStatusFieldAgentImage                           = "agentImage"
	ClusterStatusFieldAllocatable                          = "allocatable"
	ClusterStatusFieldAppliedEnableNetworkPolicy           = "appliedEnableNetworkPolicy"
	ClusterStatusFieldAppliedPodSecurityPolicyTemplateName = "appliedPodSecurityPolicyTemplateId"
	ClusterStatusFieldAppliedSpec                          = "appliedSpec"
	ClusterStatusFieldAuthImage                            = "authImage"
	ClusterStatusFieldCACert                               = "caCert"
	ClusterStatusFieldCapabilities                         = "capabilities"
	ClusterStatusFieldCapacity                             = "capacity"
	ClusterStatusFieldCertificatesExpiration               = "certificatesExpiration"
	ClusterStatusFieldComponentStatuses                    = "componentStatuses"
	ClusterStatusFieldConditions                           = "conditions"
	ClusterStatusFieldDriver                               = "driver"
	ClusterStatusFieldFailedSpec                           = "failedSpec"
	ClusterStatusFieldIstioEnabled                         = "istioEnabled"
	ClusterStatusFieldLimits                               = "limits"
	ClusterStatusFieldMonitoringStatus                     = "monitoringStatus"
	ClusterStatusFieldNodeUpgradeStatus                    = "nodeUpgradeStatus"
	ClusterStatusFieldRequested                            = "requested"
	ClusterStatusFieldVersion                              = "version"
)

type ClusterStatus struct {
	APIEndpoint                          string                    `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`
	AgentImage                           string                    `json:"agentImage,omitempty" yaml:"agentImage,omitempty"`
	Allocatable                          map[string]string         `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	AppliedEnableNetworkPolicy           bool                      `json:"appliedEnableNetworkPolicy,omitempty" yaml:"appliedEnableNetworkPolicy,omitempty"`
	AppliedPodSecurityPolicyTemplateName string                    `json:"appliedPodSecurityPolicyTemplateId,omitempty" yaml:"appliedPodSecurityPolicyTemplateId,omitempty"`
	AppliedSpec                          *ClusterSpec              `json:"appliedSpec,omitempty" yaml:"appliedSpec,omitempty"`
	AuthImage                            string                    `json:"authImage,omitempty" yaml:"authImage,omitempty"`
	CACert                               string                    `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Capabilities                         *Capabilities             `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Capacity                             map[string]string         `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	CertificatesExpiration               map[string]CertExpiration `json:"certificatesExpiration,omitempty" yaml:"certificatesExpiration,omitempty"`
	ComponentStatuses                    []ClusterComponentStatus  `json:"componentStatuses,omitempty" yaml:"componentStatuses,omitempty"`
	Conditions                           []ClusterCondition        `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Driver                               string                    `json:"driver,omitempty" yaml:"driver,omitempty"`
	FailedSpec                           *ClusterSpec              `json:"failedSpec,omitempty" yaml:"failedSpec,omitempty"`
	IstioEnabled                         bool                      `json:"istioEnabled,omitempty" yaml:"istioEnabled,omitempty"`
	Limits                               map[string]string         `json:"limits,omitempty" yaml:"limits,omitempty"`
	MonitoringStatus                     *MonitoringStatus         `json:"monitoringStatus,omitempty" yaml:"monitoringStatus,omitempty"`
	NodeUpgradeStatus                    *NodeUpgradeStatus        `json:"nodeUpgradeStatus,omitempty" yaml:"nodeUpgradeStatus,omitempty"`
	Requested                            map[string]string         `json:"requested,omitempty" yaml:"requested,omitempty"`
	Version                              *Info                     `json:"version,omitempty" yaml:"version,omitempty"`
}
