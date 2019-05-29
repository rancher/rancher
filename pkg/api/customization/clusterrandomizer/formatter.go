package clusterrandomizer

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/config"
	"k8s.io/client-go/tools/cache"
)

type Format struct {
	ClusterIndexer cache.Indexer
}

func NewFormatter(manangement *config.ScaledContext) types.Formatter {
	clusterInformer := manangement.Management.Clusters("").Controller().Informer()
	// use an indexer instead of expensive k8s api calls

	format := Format{
		ClusterIndexer: clusterInformer.GetIndexer(),
	}
	return format.Formatter
}

func (f *Format) Formatter(request *types.APIContext, resource *types.RawResource) {
}
