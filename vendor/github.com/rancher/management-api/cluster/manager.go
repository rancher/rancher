package cluster

import (
	"context"
	"net/http"
	"sync"
	"time"

	"net/url"

	"encoding/base64"

	clusterapi "github.com/rancher/cluster-api/server"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"k8s.io/client-go/rest"
)

type Manager struct {
	ManagementConfig rest.Config
	LocalConfig      *rest.Config
	servers          sync.Map
}

type record struct {
	handler http.Handler
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewManager(management *config.ManagementContext) *Manager {
	return &Manager{
		ManagementConfig: management.RESTConfig,
		LocalConfig:      management.LocalConfig,
	}
}

func (c *Manager) APIServer(ctx context.Context, cluster *client.Cluster) http.Handler {
	obj, ok := c.servers.Load(cluster.Uuid)
	if ok {
		return obj.(*record).handler
	}

	server, err := c.toServer(ctx, cluster)
	if server == nil || err != nil {
		return nil
	}

	obj, loaded := c.servers.LoadOrStore(cluster.Uuid, server)
	if !loaded {
		go func() {
			time.Sleep(10 * time.Minute)
			c.servers.Delete(cluster.Uuid)
			time.Sleep(time.Minute)
			obj.(*record).cancel()
		}()
	}

	return obj.(*record).handler
}

func (c *Manager) toRESTConfig(cluster *client.Cluster) (*rest.Config, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Internal != nil && *cluster.Internal {
		return c.LocalConfig, nil
	}

	if cluster.APIEndpoint == "" || cluster.CACert == "" || cluster.ServiceAccountToken == "" {
		return nil, nil
	}

	u, err := url.Parse(cluster.APIEndpoint)
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(cluster.CACert)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: cluster.ServiceAccountToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: data,
		},
	}, nil
}

func (c *Manager) toServer(ctx context.Context, cluster *client.Cluster) (*record, error) {
	kubeConfig, err := c.toRESTConfig(cluster)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewClusterContext(c.ManagementConfig, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}

	s := &record{}
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.handler, err = clusterapi.New(s.ctx, clusterContext)
	if err != nil {
		return nil, err
	}

	return s, nil
}
