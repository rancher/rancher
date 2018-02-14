package client

const (
	NodePoolType                = "nodePool"
	NodePoolFieldAnnotations    = "annotations"
	NodePoolFieldControlPlane   = "controlPlane"
	NodePoolFieldEtcd           = "etcd"
	NodePoolFieldHostnamePrefix = "hostnamePrefix"
	NodePoolFieldLabels         = "labels"
	NodePoolFieldNodeTemplateId = "nodeTemplateId"
	NodePoolFieldQuantity       = "quantity"
	NodePoolFieldUUID           = "uuid"
	NodePoolFieldWorker         = "worker"
)

type NodePool struct {
	Annotations    map[string]string `json:"annotations,omitempty"`
	ControlPlane   bool              `json:"controlPlane,omitempty"`
	Etcd           bool              `json:"etcd,omitempty"`
	HostnamePrefix string            `json:"hostnamePrefix,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	NodeTemplateId string            `json:"nodeTemplateId,omitempty"`
	Quantity       *int64            `json:"quantity,omitempty"`
	UUID           string            `json:"uuid,omitempty"`
	Worker         bool              `json:"worker,omitempty"`
}
