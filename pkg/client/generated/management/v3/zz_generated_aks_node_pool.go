package client

const (
	AKSNodePoolType                     = "aksNodePool"
	AKSNodePoolFieldAvailabilityZones   = "availabilityZones"
	AKSNodePoolFieldCount               = "count"
	AKSNodePoolFieldEnableAutoScaling   = "enableAutoScaling"
	AKSNodePoolFieldMaxCount            = "maxCount"
	AKSNodePoolFieldMaxPods             = "maxPods"
	AKSNodePoolFieldMaxSurge            = "maxSurge"
	AKSNodePoolFieldMinCount            = "minCount"
	AKSNodePoolFieldMode                = "mode"
	AKSNodePoolFieldName                = "name"
	AKSNodePoolFieldNodeLabels          = "nodeLabels"
	AKSNodePoolFieldNodeTaints          = "nodeTaints"
	AKSNodePoolFieldOrchestratorVersion = "orchestratorVersion"
	AKSNodePoolFieldOsDiskSizeGB        = "osDiskSizeGB"
	AKSNodePoolFieldOsDiskType          = "osDiskType"
	AKSNodePoolFieldOsType              = "osType"
	AKSNodePoolFieldVMSize              = "vmSize"
	AKSNodePoolFieldVnetSubnetID        = "vnetSubnetID"
)

type AKSNodePool struct {
	AvailabilityZones   *[]string         `json:"availabilityZones,omitempty" yaml:"availabilityZones,omitempty"`
	Count               *int64            `json:"count,omitempty" yaml:"count,omitempty"`
	EnableAutoScaling   *bool             `json:"enableAutoScaling,omitempty" yaml:"enableAutoScaling,omitempty"`
	MaxCount            *int64            `json:"maxCount,omitempty" yaml:"maxCount,omitempty"`
	MaxPods             *int64            `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	MaxSurge            string            `json:"maxSurge,omitempty" yaml:"maxSurge,omitempty"`
	MinCount            *int64            `json:"minCount,omitempty" yaml:"minCount,omitempty"`
	Mode                string            `json:"mode,omitempty" yaml:"mode,omitempty"`
	Name                *string           `json:"name,omitempty" yaml:"name,omitempty"`
	NodeLabels          map[string]string `json:"nodeLabels,omitempty" yaml:"nodeLabels,omitempty"`
	NodeTaints          []string          `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	OrchestratorVersion *string           `json:"orchestratorVersion,omitempty" yaml:"orchestratorVersion,omitempty"`
	OsDiskSizeGB        *int64            `json:"osDiskSizeGB,omitempty" yaml:"osDiskSizeGB,omitempty"`
	OsDiskType          string            `json:"osDiskType,omitempty" yaml:"osDiskType,omitempty"`
	OsType              string            `json:"osType,omitempty" yaml:"osType,omitempty"`
	VMSize              string            `json:"vmSize,omitempty" yaml:"vmSize,omitempty"`
	VnetSubnetID        *string           `json:"vnetSubnetID,omitempty" yaml:"vnetSubnetID,omitempty"`
}
