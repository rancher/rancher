package kontainerdriver

import (
	"fmt"

	errorsutil "github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

type store struct {
	types.Store
	KontainerDriverLister v3.KontainerDriverLister
	ClusterIndexer        cache.Indexer
}

func NewStore(management *config.ScaledContext, s types.Store) types.Store {
	clusterInformer := management.Management.Clusters("").Controller().Informer()
	kd := management.Management.KontainerDrivers("").Controller().Lister()
	storeObj := store{
		Store:                 s,
		KontainerDriverLister: kd,
		ClusterIndexer:        clusterInformer.GetIndexer(),
	}
	return &storeObj
}

// Delete removes the KontainerDriver if it is not in use by a cluster
func (s *store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	//need to get the full driver since just the id is not enough see if it is builtin
	driver, err := s.KontainerDriverLister.Get("", id)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, errorsutil.WithMessage(err, fmt.Sprintf("error getting kontainer driver %s", id))
		}
		//if driver is not found, don't return error
		return nil, nil
	}
	if driver.Spec.BuiltIn {
		return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "builtin cluster drivers may not be removed")
	}
	clustersWithKontainerDriver, err := s.ClusterIndexer.ByIndex(clusterByGenericEngineConfigKey, id)
	if err != nil {
		return nil, errorsutil.WithMessage(err, fmt.Sprintf("error determining if kontainer driver [%s] was in use", driver.Status.DisplayName))
	}

	if len(clustersWithKontainerDriver) != 0 {
		return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "cluster Driver is in use by one or more clusters, delete clusters before deleting this driver")
	}
	return s.Store.Delete(apiContext, schema, id)
}
