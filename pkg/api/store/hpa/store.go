package hpa

import (
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

type NoWatchByClusterVersionStore struct {
	types.Store
	ClusterLister mgmtv3.ClusterLister
	Manager       *clustermanager.Manager
}

func (s *NoWatchByClusterVersionStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	clusterName := s.Manager.ClusterName(apiContext)
	obj, err := s.ClusterLister.Get("", clusterName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cluster obj from lister")
	}
	version := obj.Status.Version
	// autoscaing v2beta2 was released in v1.12 https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG-1.12.md#sig-autoscaling
	if version == nil || version.Minor < "12" {
		return nil, nil
	}

	return s.Store.Watch(apiContext, schema, opt)
}
