package client

const (
	AliNodePoolType                    = "aliNodePool"
	AliNodePoolFieldAutoRenew          = "autoRenew"
	AliNodePoolFieldAutoRenewPeriod    = "autoRenewPeriod"
	AliNodePoolFieldDataDisks          = "dataDisks"
	AliNodePoolFieldDesiredSize        = "desiredSize"
	AliNodePoolFieldEnableAutoScaling  = "enableAutoScaling"
	AliNodePoolFieldImageID            = "imageId"
	AliNodePoolFieldImageType          = "imageType"
	AliNodePoolFieldInstanceChargeType = "instanceChargeType"
	AliNodePoolFieldInstanceTypes      = "instanceTypes"
	AliNodePoolFieldKeyPair            = "keyPair"
	AliNodePoolFieldMaxInstances       = "maxInstances"
	AliNodePoolFieldMinInstances       = "minInstances"
	AliNodePoolFieldName               = "name"
	AliNodePoolFieldNodePoolID         = "nodePoolId"
	AliNodePoolFieldPeriod             = "period"
	AliNodePoolFieldPeriodUnit         = "periodUnit"
	AliNodePoolFieldRuntime            = "runtime"
	AliNodePoolFieldRuntimeVersion     = "runtimeVersion"
	AliNodePoolFieldScalingType        = "scalingType"
	AliNodePoolFieldSystemDiskCategory = "systemDiskCategory"
	AliNodePoolFieldSystemDiskSize     = "systemDiskSize"
	AliNodePoolFieldVSwitchIDs         = "vSwitchIds"
)

type AliNodePool struct {
	AutoRenew          bool      `json:"autoRenew,omitempty" yaml:"autoRenew,omitempty"`
	AutoRenewPeriod    int64     `json:"autoRenewPeriod,omitempty" yaml:"autoRenewPeriod,omitempty"`
	DataDisks          []AliDisk `json:"dataDisks,omitempty" yaml:"dataDisks,omitempty"`
	DesiredSize        *int64    `json:"desiredSize,omitempty" yaml:"desiredSize,omitempty"`
	EnableAutoScaling  *bool     `json:"enableAutoScaling,omitempty" yaml:"enableAutoScaling,omitempty"`
	ImageID            string    `json:"imageId,omitempty" yaml:"imageId,omitempty"`
	ImageType          string    `json:"imageType,omitempty" yaml:"imageType,omitempty"`
	InstanceChargeType string    `json:"instanceChargeType,omitempty" yaml:"instanceChargeType,omitempty"`
	InstanceTypes      []string  `json:"instanceTypes,omitempty" yaml:"instanceTypes,omitempty"`
	KeyPair            string    `json:"keyPair,omitempty" yaml:"keyPair,omitempty"`
	MaxInstances       *int64    `json:"maxInstances,omitempty" yaml:"maxInstances,omitempty"`
	MinInstances       *int64    `json:"minInstances,omitempty" yaml:"minInstances,omitempty"`
	Name               string    `json:"name,omitempty" yaml:"name,omitempty"`
	NodePoolID         string    `json:"nodePoolId,omitempty" yaml:"nodePoolId,omitempty"`
	Period             int64     `json:"period,omitempty" yaml:"period,omitempty"`
	PeriodUnit         string    `json:"periodUnit,omitempty" yaml:"periodUnit,omitempty"`
	Runtime            string    `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	RuntimeVersion     string    `json:"runtimeVersion,omitempty" yaml:"runtimeVersion,omitempty"`
	ScalingType        string    `json:"scalingType,omitempty" yaml:"scalingType,omitempty"`
	SystemDiskCategory string    `json:"systemDiskCategory,omitempty" yaml:"systemDiskCategory,omitempty"`
	SystemDiskSize     int64     `json:"systemDiskSize,omitempty" yaml:"systemDiskSize,omitempty"`
	VSwitchIDs         []string  `json:"vSwitchIds,omitempty" yaml:"vSwitchIds,omitempty"`
}
