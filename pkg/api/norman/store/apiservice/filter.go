package apiservice

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	apiregistrationv1 "github.com/rancher/rancher/pkg/generated/norman/apiregistration.k8s.io/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type noWatchByAPIServiceStore struct {
	types.Store
	manager *clustermanager.Manager
	version string
}

func NewAPIServicFilterStoreFunc(cm *clustermanager.Manager, apiVersion string) func(types.Store) types.Store {
	return func(store types.Store) types.Store {
		return &noWatchByAPIServiceStore{
			Store:   store,
			manager: cm,
			version: apiVersion,
		}
	}
}

func (s *noWatchByAPIServiceStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	clustername := s.manager.ClusterName(apiContext)
	versionName := getAPIVersionName(s.version)
	lister, err := s.getAPIServiceLister(clustername)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get api service lister in cluster %s", clustername)
	}
	if _, err := lister.Get("", versionName); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get apiservice %s from lister in cluster %s", versionName, clustername)
		}
		return nil, nil
	}
	return s.Store.Watch(apiContext, schema, opt)
}

func (s *noWatchByAPIServiceStore) getAPIServiceLister(clusterName string) (apiregistrationv1.APIServiceLister, error) {
	userContext, err := s.manager.UserContext(clusterName)
	if err != nil {
		return nil, err
	}
	return userContext.APIAggregation.APIServices("").Controller().Lister(), nil
}

func getAPIVersionName(version string) string {
	parts := strings.Split(version, "/")
	if len(parts) == 1 {
		parts[0] = parts[0] + "."
	}
	//reverse the splited strings, [autoscaling,v2beta2] become [v2beta2,autoscaling]
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}
