package v3

import (
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectResourceQuota struct {
	Limit     ProjectResourceLimit `json:"limit,omitempty"`
	UsedLimit ProjectResourceLimit `json:"usedLimit,omitempty"`
}

type ProjectResourceLimit struct {
	Pods                   string `json:"pods,omitempty"`
	Services               string `json:"services,omitempty"`
	ReplicationControllers string `json:"replicationControllers,omitempty"`
	Secrets                string `json:"secrets,omitempty"`
	ConfigMaps             string `json:"configMaps,omitempty"`
	PersistentVolumeClaims string `json:"persistentVolumeClaims,omitempty"`
	ServicesNodePorts      string `json:"servicesNodePorts,omitempty"`
	ServicesLoadBalancers  string `json:"servicesLoadBalancers,omitempty"`
	RequestsCPU            string `json:"requestsCpu,omitempty"`
	RequestsMemory         string `json:"requestsMemory,omitempty"`
	RequestsStorage        string `json:"requestsStorage,omitempty"`
	LimitsCPU              string `json:"limitsCpu,omitempty"`
	LimitsMemory           string `json:"limitsMemory,omitempty"`
}

type ResourceQuotaTemplate struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string `json:"description"`
	IsDefault   bool   `json:"isDefault"`
	ClusterName string `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`

	ProjectResourceQuota
}
