package client

const (
	NodePoolSpecType                 = "nodePoolSpec"
	NodePoolSpecFieldClusterId       = "clusterId"
	NodePoolSpecFieldControlPlane    = "controlPlane"
	NodePoolSpecFieldDisplayName     = "displayName"
	NodePoolSpecFieldEtcd            = "etcd"
	NodePoolSpecFieldHostnamePrefix  = "hostnamePrefix"
	NodePoolSpecFieldNodeAnnotations = "nodeAnnotations"
	NodePoolSpecFieldNodeLabels      = "nodeLabels"
	NodePoolSpecFieldNodeTemplateId  = "nodeTemplateId"
	NodePoolSpecFieldQuantity        = "quantity"
	NodePoolSpecFieldWorker          = "worker"
)

type NodePoolSpec struct {
	ClusterId       string            `json:"clusterId,omitempty"`
	ControlPlane    bool              `json:"controlPlane,omitempty"`
	DisplayName     string            `json:"displayName,omitempty"`
	Etcd            bool              `json:"etcd,omitempty"`
	HostnamePrefix  string            `json:"hostnamePrefix,omitempty"`
	NodeAnnotations map[string]string `json:"nodeAnnotations,omitempty"`
	NodeLabels      map[string]string `json:"nodeLabels,omitempty"`
	NodeTemplateId  string            `json:"nodeTemplateId,omitempty"`
	Quantity        *int64            `json:"quantity,omitempty"`
	Worker          bool              `json:"worker,omitempty"`
}
