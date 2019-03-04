package kontainerdriver

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

type Format struct {
	ClusterIndexer cache.Indexer
}

func NewFormatter(manangement *config.ScaledContext) types.Formatter {
	clusterInformer := manangement.Management.Clusters("").Controller().Informer()
	// use an indexer instead of expensive k8s api calls
	clusterInformer.AddIndexers(map[string]cache.IndexFunc{
		clusterByKontainerDriverKey: clusterByKontainerDriver,
	})

	format := Format{
		ClusterIndexer: clusterInformer.GetIndexer(),
	}
	return format.Formatter
}

const clusterByKontainerDriverKey = "clusterbyKontainerDriver"

func clusterByKontainerDriver(obj interface{}) ([]string, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		return []string{}, nil
	}
	return []string{cluster.Status.Driver}, nil
}
func (f *Format) Formatter(request *types.APIContext, resource *types.RawResource) {
	state, ok := resource.Values["state"].(string)
	if ok {
		if state == "active" {
			resource.AddAction(request, "deactivate")
		}

		if state == "inactive" {
			resource.AddAction(request, "activate")
		}
	}
	// if cluster driver is a built-in, delete removal link from UI
	if builtIn, _ := resource.Values[client.KontainerDriverFieldBuiltIn].(bool); builtIn {
		delete(resource.Links, "remove")
	}
	resName := resource.Values[client.KontainerDriverFieldName]
	// resName will be nil when first added
	if resName != nil {
		clustersWithKontainerDriver, err := f.ClusterIndexer.ByIndex(clusterByKontainerDriverKey, resName.(string))
		if err != nil {
			logrus.Warnf("failed to determine if kontainer driver %v was in use by a cluster : %v", resName.(string), err)
		} else if len(clustersWithKontainerDriver) != 0 {
			// if cluster driver in use, delete removal link from UI
			delete(resource.Links, "remove")
		}
	}
}
