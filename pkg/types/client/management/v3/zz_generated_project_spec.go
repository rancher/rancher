package client

const (
	ProjectSpecType                               = "projectSpec"
	ProjectSpecFieldClusterID                     = "clusterId"
	ProjectSpecFieldContainerDefaultResourceLimit = "containerDefaultResourceLimit"
	ProjectSpecFieldDescription                   = "description"
	ProjectSpecFieldDisplayName                   = "displayName"
	ProjectSpecFieldEnableProjectMonitoring       = "enableProjectMonitoring"
	ProjectSpecFieldNamespaceDefaultResourceQuota = "namespaceDefaultResourceQuota"
	ProjectSpecFieldResourceQuota                 = "resourceQuota"
)

type ProjectSpec struct {
	ClusterID                     string                  `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ContainerDefaultResourceLimit *ContainerResourceLimit `json:"containerDefaultResourceLimit,omitempty" yaml:"containerDefaultResourceLimit,omitempty"`
	Description                   string                  `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName                   string                  `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	EnableProjectMonitoring       bool                    `json:"enableProjectMonitoring,omitempty" yaml:"enableProjectMonitoring,omitempty"`
	NamespaceDefaultResourceQuota *NamespaceResourceQuota `json:"namespaceDefaultResourceQuota,omitempty" yaml:"namespaceDefaultResourceQuota,omitempty"`
	ResourceQuota                 *ProjectResourceQuota   `json:"resourceQuota,omitempty" yaml:"resourceQuota,omitempty"`
}
