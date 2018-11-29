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
	queryAction           = "query"
	querycluster          = "querycluster"
	queryproject          = "queryproject"
	listclustermetricname = "listclustermetricname"
	listprojectmetricname = "listprojectmetricname"
)

func GraphFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, queryAction)
}

func QueryGraphCollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, queryAction)
}

func MetricFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, querycluster)
	resource.AddAction(apiContext, queryproject)
	resource.AddAction(apiContext, listclustermetricname)
	resource.AddAction(apiContext, listprojectmetricname)
}

func MetricCollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, querycluster)
	collection.AddAction(apiContext, queryproject)
	collection.AddAction(apiContext, listclustermetricname)
	collection.AddAction(apiContext, listprojectmetricname)
}
