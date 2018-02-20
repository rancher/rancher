package clusterrouter

import (
	"net/http"
	"sync"

	"github.com/docker/docker/pkg/locker"
	"github.com/rancher/rancher/pkg/clusterrouter/proxy"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

type factory struct {
	dialerFactory dialer.Factory
	clusterLookup ClusterLookup
	clusters      sync.Map
	serverLock    *locker.Locker
	servers       sync.Map
}

func newFactory(dialer dialer.Factory, lookup ClusterLookup) *factory {
	return &factory{
		dialerFactory: dialer,
		serverLock:    locker.New(),
		clusterLookup: lookup,
	}
}

func (s *factory) lookupCluster(clusterID string) (*v3.Cluster, http.Handler) {
	srv, ok := s.servers.Load(clusterID)
	if ok {
		if cluster, ok := s.clusters.Load(clusterID); ok {
			return cluster.(*v3.Cluster), srv.(server).Handler()
		}
	}

	return nil, nil
}

func (s *factory) get(req *http.Request) (*v3.Cluster, http.Handler, error) {
	cluster, err := s.clusterLookup.Lookup(req)
	if err != nil || cluster == nil {
		return nil, nil, err
	}
	clusterID := cluster.Name

	if newCluster, handler := s.lookupCluster(clusterID); newCluster != nil {
		return newCluster, handler, nil
	}

	s.serverLock.Lock("cluster." + clusterID)
	defer s.serverLock.Unlock("cluster." + clusterID)

	if newCluster, handler := s.lookupCluster(clusterID); newCluster != nil {
		return newCluster, handler, nil
	}

	var srv interface{}
	srv, err = s.newServer(cluster)
	if err != nil || srv == nil {
		return nil, nil, err
	}

	srv, _ = s.servers.LoadOrStore(cluster.Name, srv)
	s.clusters.LoadOrStore(cluster.Name, cluster)

	return cluster, srv.(server).Handler(), nil
}

func (s *factory) newServer(c *v3.Cluster) (server, error) {
	return proxy.New(c, s.dialerFactory)
}
