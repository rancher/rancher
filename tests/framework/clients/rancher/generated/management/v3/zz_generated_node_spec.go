package client

const (
	NodeSpecType                          = "nodeSpec"
	NodeSpecFieldControlPlane             = "controlPlane"
	NodeSpecFieldCustomConfig             = "customConfig"
	NodeSpecFieldDescription              = "description"
	NodeSpecFieldDesiredNodeTaints        = "desiredNodeTaints"
	NodeSpecFieldDesiredNodeUnschedulable = "desiredNodeUnschedulable"
	NodeSpecFieldDisplayName              = "displayName"
	NodeSpecFieldEtcd                     = "etcd"
	NodeSpecFieldImported                 = "imported"
	NodeSpecFieldMetadataUpdate           = "metadataUpdate"
	NodeSpecFieldNodeDrainInput           = "nodeDrainInput"
	NodeSpecFieldNodePoolID               = "nodePoolId"
	NodeSpecFieldNodeTemplateID           = "nodeTemplateId"
	NodeSpecFieldPodCidr                  = "podCidr"
	NodeSpecFieldPodCidrs                 = "podCidrs"
	NodeSpecFieldProviderId               = "providerId"
	NodeSpecFieldRequestedHostname        = "requestedHostname"
	NodeSpecFieldScaledownTime            = "scaledownTime"
	NodeSpecFieldTaints                   = "taints"
	NodeSpecFieldUnschedulable            = "unschedulable"
	NodeSpecFieldUpdateTaintsFromAPI      = "updateTaintsFromAPI"
	NodeSpecFieldWorker                   = "worker"
)

type NodeSpec struct {
	ControlPlane             bool            `json:"controlPlane,omitempty" yaml:"controlPlane,omitempty"`
	CustomConfig             *CustomConfig   `json:"customConfig,omitempty" yaml:"customConfig,omitempty"`
	Description              string          `json:"description,omitempty" yaml:"description,omitempty"`
	DesiredNodeTaints        []Taint         `json:"desiredNodeTaints,omitempty" yaml:"desiredNodeTaints,omitempty"`
	DesiredNodeUnschedulable string          `json:"desiredNodeUnschedulable,omitempty" yaml:"desiredNodeUnschedulable,omitempty"`
	DisplayName              string          `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Etcd                     bool            `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Imported                 bool            `json:"imported,omitempty" yaml:"imported,omitempty"`
	MetadataUpdate           *MetadataUpdate `json:"metadataUpdate,omitempty" yaml:"metadataUpdate,omitempty"`
	NodeDrainInput           *NodeDrainInput `json:"nodeDrainInput,omitempty" yaml:"nodeDrainInput,omitempty"`
	NodePoolID               string          `json:"nodePoolId,omitempty" yaml:"nodePoolId,omitempty"`
	NodeTemplateID           string          `json:"nodeTemplateId,omitempty" yaml:"nodeTemplateId,omitempty"`
	PodCidr                  string          `json:"podCidr,omitempty" yaml:"podCidr,omitempty"`
	PodCidrs                 []string        `json:"podCidrs,omitempty" yaml:"podCidrs,omitempty"`
	ProviderId               string          `json:"providerId,omitempty" yaml:"providerId,omitempty"`
	RequestedHostname        string          `json:"requestedHostname,omitempty" yaml:"requestedHostname,omitempty"`
	ScaledownTime            string          `json:"scaledownTime,omitempty" yaml:"scaledownTime,omitempty"`
	Taints                   []Taint         `json:"taints,omitempty" yaml:"taints,omitempty"`
	Unschedulable            bool            `json:"unschedulable,omitempty" yaml:"unschedulable,omitempty"`
	UpdateTaintsFromAPI      *bool           `json:"updateTaintsFromAPI,omitempty" yaml:"updateTaintsFromAPI,omitempty"`
	Worker                   bool            `json:"worker,omitempty" yaml:"worker,omitempty"`
}
