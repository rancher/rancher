package client

const (
	NodePoolSpecType                         = "nodePoolSpec"
	NodePoolSpecFieldClusterID               = "clusterId"
	NodePoolSpecFieldControlPlane            = "controlPlane"
	NodePoolSpecFieldDeleteNotReadyAfterSecs = "deleteNotReadyAfterSecs"
	NodePoolSpecFieldDisplayName             = "displayName"
	NodePoolSpecFieldDrainBeforeDelete       = "drainBeforeDelete"
	NodePoolSpecFieldEtcd                    = "etcd"
	NodePoolSpecFieldHostnamePrefix          = "hostnamePrefix"
	NodePoolSpecFieldNodeAnnotations         = "nodeAnnotations"
	NodePoolSpecFieldNodeLabels              = "nodeLabels"
	NodePoolSpecFieldNodeTaints              = "nodeTaints"
	NodePoolSpecFieldNodeTemplateID          = "nodeTemplateId"
	NodePoolSpecFieldQuantity                = "quantity"
	NodePoolSpecFieldWorker                  = "worker"
)

type NodePoolSpec struct {
	ClusterID               string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ControlPlane            bool              `json:"controlPlane,omitempty" yaml:"controlPlane,omitempty"`
	DeleteNotReadyAfterSecs int64             `json:"deleteNotReadyAfterSecs,omitempty" yaml:"deleteNotReadyAfterSecs,omitempty"`
	DisplayName             string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	DrainBeforeDelete       bool              `json:"drainBeforeDelete,omitempty" yaml:"drainBeforeDelete,omitempty"`
	Etcd                    bool              `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	HostnamePrefix          string            `json:"hostnamePrefix,omitempty" yaml:"hostnamePrefix,omitempty"`
	NodeAnnotations         map[string]string `json:"nodeAnnotations,omitempty" yaml:"nodeAnnotations,omitempty"`
	NodeLabels              map[string]string `json:"nodeLabels,omitempty" yaml:"nodeLabels,omitempty"`
	NodeTaints              []Taint           `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	NodeTemplateID          string            `json:"nodeTemplateId,omitempty" yaml:"nodeTemplateId,omitempty"`
	Quantity                int64             `json:"quantity,omitempty" yaml:"quantity,omitempty"`
	Worker                  bool              `json:"worker,omitempty" yaml:"worker,omitempty"`
}
