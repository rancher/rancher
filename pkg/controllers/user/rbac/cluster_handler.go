package rbac

import (
	"reflect"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	grbByRoleIndex = "management.cattle.io/grb-by-role"
)

func newClusterHandler(workload *config.UserContext) v3.ClusterHandlerFunc { //*clusterHandler {
	informer := workload.Management.Management.GlobalRoleBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		grbByRoleIndex: grbByRole,
	}
	informer.AddIndexers(indexers)

	ch := &clusterHandler{
		grbIndexer:    informer.GetIndexer(),
		grbController: workload.Management.Management.GlobalRoleBindings("").Controller(),
		clusters:      workload.Management.Management.Clusters(""),
		clusterName:   workload.ClusterName,
	}
	return ch.sync
}

type clusterHandler struct {
	grbController v3.GlobalRoleBindingController
	grbIndexer    cache.Indexer
	clusters      v3.ClusterInterface
	clusterName   string
}

func (h *clusterHandler) sync(key string, cluster *v3.Cluster) error {
	if key == "" || cluster == nil || cluster.Name != h.clusterName {
		return nil
	}

	original := cluster
	cluster = original.DeepCopy()

	var updateErr error
	err := h.doSync(cluster)
	if cluster != nil && !reflect.DeepEqual(cluster, original) {
		_, updateErr = h.clusters.Update(cluster)
	}

	if err != nil {
		return err
	}
	return updateErr
}

func (h *clusterHandler) doSync(cluster *v3.Cluster) error {
	_, err := v3.ClusterConditionGlobalAdminsSynced.DoUntilTrue(cluster, func() (runtime.Object, error) {
		grbs, err := h.grbIndexer.ByIndex(grbByRoleIndex, "admin")
		if err != nil {
			return cluster, err
		}

		for _, x := range grbs {
			grb, _ := x.(*v3.GlobalRoleBinding)
			h.grbController.Enqueue("", grb.Name)
		}

		return cluster, nil
	})
	return err
}

func grbByRole(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{grb.GlobalRoleName}, nil
}
