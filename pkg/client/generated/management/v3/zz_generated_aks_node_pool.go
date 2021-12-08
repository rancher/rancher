package client

const (
	AKSNodePoolType                     = "aksNodePool"
	AKSNodePoolFieldAvailabilityZones   = "availabilityZones"
	AKSNodePoolFieldCount               = "count"
	AKSNodePoolFieldEnableAutoScaling   = "enableAutoScaling"
	AKSNodePoolFieldMaxCount            = "maxCount"
	AKSNodePoolFieldMaxPods             = "maxPods"
	AKSNodePoolFieldMinCount            = "minCount"
	AKSNodePoolFieldMode                = "mode"
	AKSNodePoolFieldName                = "name"
	AKSNodePoolFieldOrchestratorVersion = "orchestratorVersion"
	AKSNodePoolFieldOsDiskSizeGB        = "osDiskSizeGB"
	AKSNodePoolFieldOsDiskType          = "osDiskType"
	AKSNodePoolFieldOsType              = "osType"
	AKSNodePoolFieldVMSize              = "vmSize"
)

type AKSNodePool struct {
	AvailabilityZones   *[]string `json:"availabilityZones,omitempty" yaml:"availabilityZones,omitempty"`
	Count               *int64    `json:"count,omitempty" yaml:"count,omitempty"`
	EnableAutoScaling   *bool     `json:"enableAutoScaling,omitempty" yaml:"enableAutoScaling,omitempty"`
	MaxCount            *int64    `json:"maxCount,omitempty" yaml:"maxCount,omitempty"`
	MaxPods             *int64    `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	MinCount            *int64    `json:"minCount,omitempty" yaml:"minCount,omitempty"`
	Mode                string    `json:"mode,omitempty" yaml:"mode,omitempty"`
	Name                *string   `json:"name,omitempty" yaml:"name,omitempty"`
	OrchestratorVersion *string   `json:"orchestratorVersion,omitempty" yaml:"orchestratorVersion,omitempty"`
	OsDiskSizeGB        *int64    `json:"osDiskSizeGB,omitempty" yaml:"osDiskSizeGB,omitempty"`
	OsDiskType          string    `json:"osDiskType,omitempty" yaml:"osDiskType,omitempty"`
	OsType              string    `json:"osType,omitempty" yaml:"osType,omitempty"`
	VMSize              string    `json:"vmSize,omitempty" yaml:"vmSize,omitempty"`
}
