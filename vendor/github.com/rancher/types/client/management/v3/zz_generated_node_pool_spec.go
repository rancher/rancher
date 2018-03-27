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
	ClusterId       string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ControlPlane    bool              `json:"controlPlane,omitempty" yaml:"controlPlane,omitempty"`
	DisplayName     string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Etcd            bool              `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	HostnamePrefix  string            `json:"hostnamePrefix,omitempty" yaml:"hostnamePrefix,omitempty"`
	NodeAnnotations map[string]string `json:"nodeAnnotations,omitempty" yaml:"nodeAnnotations,omitempty"`
	NodeLabels      map[string]string `json:"nodeLabels,omitempty" yaml:"nodeLabels,omitempty"`
	NodeTemplateId  string            `json:"nodeTemplateId,omitempty" yaml:"nodeTemplateId,omitempty"`
	Quantity        int64             `json:"quantity,omitempty" yaml:"quantity,omitempty"`
	Worker          bool              `json:"worker,omitempty" yaml:"worker,omitempty"`
}
