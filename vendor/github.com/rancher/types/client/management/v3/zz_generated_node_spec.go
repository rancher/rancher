package client

const (
	NodeSpecType                   = "nodeSpec"
	NodeSpecFieldClusterId         = "clusterId"
	NodeSpecFieldControlPlane      = "controlPlane"
	NodeSpecFieldCustomConfig      = "customConfig"
	NodeSpecFieldDescription       = "description"
	NodeSpecFieldDisplayName       = "displayName"
	NodeSpecFieldEtcd              = "etcd"
	NodeSpecFieldImported          = "imported"
	NodeSpecFieldNodePoolUUID      = "nodePoolUuid"
	NodeSpecFieldNodeTemplateId    = "nodeTemplateId"
	NodeSpecFieldPodCidr           = "podCidr"
	NodeSpecFieldProviderId        = "providerId"
	NodeSpecFieldRequestedHostname = "requestedHostname"
	NodeSpecFieldTaints            = "taints"
	NodeSpecFieldUnschedulable     = "unschedulable"
	NodeSpecFieldWorker            = "worker"
)

type NodeSpec struct {
	ClusterId         string        `json:"clusterId,omitempty"`
	ControlPlane      *bool         `json:"controlPlane,omitempty"`
	CustomConfig      *CustomConfig `json:"customConfig,omitempty"`
	Description       string        `json:"description,omitempty"`
	DisplayName       string        `json:"displayName,omitempty"`
	Etcd              *bool         `json:"etcd,omitempty"`
	Imported          *bool         `json:"imported,omitempty"`
	NodePoolUUID      string        `json:"nodePoolUuid,omitempty"`
	NodeTemplateId    string        `json:"nodeTemplateId,omitempty"`
	PodCidr           string        `json:"podCidr,omitempty"`
	ProviderId        string        `json:"providerId,omitempty"`
	RequestedHostname string        `json:"requestedHostname,omitempty"`
	Taints            []Taint       `json:"taints,omitempty"`
	Unschedulable     *bool         `json:"unschedulable,omitempty"`
	Worker            *bool         `json:"worker,omitempty"`
}
