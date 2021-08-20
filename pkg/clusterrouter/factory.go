package clusterrouter

import (
	"net/http"
	"sync"

	"github.com/moby/locker"
	"github.com/rancher/rancher/pkg/clusterrouter/proxy"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"k8s.io/client-go/rest"
)

type factory struct {
	dialerFactory        dialer.Factory
	clusterLookup        ClusterLookup
	clusterLister        v3.ClusterLister
	clusters             sync.Map
	serverLock           *locker.Locker
	servers              sync.Map
	localConfig          *rest.Config
	clusterContextGetter proxy.ClusterContextGetter
}

func newFactory(localConfig *rest.Config, dialer dialer.Factory, lookup ClusterLookup, clusterLister v3.ClusterLister, clusterContextGetter proxy.ClusterContextGetter) *factory {
	return &factory{
		dialerFactory:        dialer,
		serverLock:           locker.New(),
		clusterLookup:        lookup,
		clusterLister:        clusterLister,
		localConfig:          localConfig,
		clusterContextGetter: clusterContextGetter,
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
	return proxy.New(s.localConfig, c, s.clusterLister, s.dialerFactory, s.clusterContextGetter)
}
