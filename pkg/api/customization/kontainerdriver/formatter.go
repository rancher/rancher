package kontainerdriver

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
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
		clusterByGenericEngineConfigKey: clusterByKontainerDriver,
	})

	format := Format{
		ClusterIndexer: clusterInformer.GetIndexer(),
	}
	return format.Formatter
}

func CollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, "refresh")
	currContext := apiContext.URLBuilder.Current()
	if !strings.HasSuffix(currContext, "/") {
		currContext = fmt.Sprintf("%s/", currContext)
	}
	collection.Links["rancher-images"] = fmt.Sprintf("%srancher-images", currContext)
	collection.Links["rancher-windows-images"] = fmt.Sprintf("%srancher-windows-images", currContext)
}

const clusterByGenericEngineConfigKey = "genericEngineConfig"

// clusterByKontainerDriver is an indexer function that uses the cluster genericEngineConfig
// driverName field
func clusterByKontainerDriver(obj interface{}) ([]string, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		return []string{}, nil
	}
	engineConfig := cluster.Spec.GenericEngineConfig
	if engineConfig == nil {
		return []string{}, nil
	}
	driverName, ok := (*engineConfig)["driverName"].(string)
	if !ok {
		return []string{}, nil
	}

	return []string{driverName}, nil
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
		return
	}
	resName := resource.Values["id"]
	// resName will be nil when first added
	if resName != nil {
		clustersWithKontainerDriver, err := f.ClusterIndexer.ByIndex(clusterByGenericEngineConfigKey, resName.(string))
		if err != nil {
			logrus.Warnf("failed to determine if kontainer driver %v was in use by a cluster : %v", resName.(string), err)
		} else if len(clustersWithKontainerDriver) != 0 {
			// if cluster driver in use, delete removal link from UI
			delete(resource.Links, "remove")
		}
	}
}
