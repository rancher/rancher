package server

import (
	"net/http"
	"sync"

	"github.com/docker/docker/pkg/locker"
	"github.com/rancher/netes/cluster"
	"github.com/rancher/netes/server/proxy"
	"github.com/rancher/netes/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type Factory struct {
	dialerFactory types.DialerFactory
	clusterLookup cluster.Lookup
	clusters      sync.Map
	config        *types.GlobalConfig
	serverLock    *locker.Locker
	servers       sync.Map
}

func NewFactory(config *types.GlobalConfig) *Factory {
	return &Factory{
		dialerFactory: config.DialerFactory,
		serverLock:    locker.New(),
		config:        config,
		clusterLookup: config.Lookup,
	}
}

func (s *Factory) lookupCluster(clusterID string) (*v3.Cluster, http.Handler) {
	server, ok := s.servers.Load(clusterID)
	if ok {
		if cluster, ok := s.clusters.Load(clusterID); ok {
			return cluster.(*v3.Cluster), server.(Server).Handler()
		}
	}

	return nil, nil
}

func (s *Factory) Get(req *http.Request) (*v3.Cluster, http.Handler, error) {
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

	var server interface{}
	server, err = s.newServer(cluster)
	if err != nil || server == nil {
		return nil, nil, err
	}

	server, _ = s.servers.LoadOrStore(cluster.Name, server)
	s.clusters.LoadOrStore(cluster.Name, cluster)

	return cluster, server.(Server).Handler(), nil
}

func (s *Factory) newServer(c *v3.Cluster) (Server, error) {
	if c.Spec.EmbeddedConfig != nil {
		//return embedded.New(s.config, c, s.config.Lookup)
		return nil, nil
	}

	if c.Spec.Internal {

	} else {
		return proxy.New(c, s.dialerFactory)
	}

	return nil, nil
}
