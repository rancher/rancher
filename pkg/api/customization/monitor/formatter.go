package monitor

import (
	"github.com/rancher/norman/types"
)

const (
	ResourceNode              = "node"
	ResourceCluster           = "cluster"
	ResourceEtcd              = "etcd"
	ResourceAPIServer         = "apiserver"
	ResourceScheduler         = "scheduler"
	ResourceControllerManager = "controllermanager"
	ResourceIngressController = "ingressController"
	ResourceFluentd           = "fluentd"
	ResourceWorkload          = "workload"
	ResourcePod               = "pod"
	ResourceContainer         = "container"
)

const (
	queryAction    = "query"
	querycluster   = "querycluster"
	queryproject   = "queryproject"
	listmetricname = "listmetricname"
)

func QueryGraphCollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, queryAction)
}

func MetricCollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, querycluster)
	collection.AddAction(apiContext, queryproject)
	collection.AddAction(apiContext, listmetricname)
}
