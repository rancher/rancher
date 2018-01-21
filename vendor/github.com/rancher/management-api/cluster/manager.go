package cluster

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"sync"
	"time"

	clusterapi "github.com/rancher/cluster-api/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
)

type Manager struct {
	ClusterLister    v3.ClusterLister
	ManagementConfig rest.Config
	LocalConfig      *rest.Config
	servers          sync.Map
}

type record struct {
	handler        http.Handler
	cluster        *v3.Cluster
	clusterContext *config.ClusterContext
	ctx            context.Context
	cancel         context.CancelFunc
}

func (r *record) start() {
	r.clusterContext.Start(r.ctx)
}

func NewManager(management *config.ManagementContext) *Manager {
	return &Manager{
		ClusterLister:    management.Management.Clusters("").Controller().Lister(),
		ManagementConfig: management.RESTConfig,
		LocalConfig:      management.LocalConfig,
	}
}

func (c *Manager) APIServer(ctx context.Context, cluster *client.Cluster) http.Handler {
	obj, ok := c.servers.Load(cluster.Uuid)
	if ok {
		r := obj.(*record)
		if !c.changed(r) {
			return obj.(*record).handler
		}
		c.stop(r)
	}

	server, err := c.toServer(cluster)
	if server == nil || err != nil {
		if err != nil {
			logrus.Errorf("Failed to load cluster %s: %v", cluster.ID, err)
		}
		return nil
	}

	obj, loaded := c.servers.LoadOrStore(cluster.Uuid, server)
	if !loaded {
		r := obj.(*record)
		go r.start()
		go c.watch(r)
	}

	return obj.(*record).handler
}

func (c *Manager) changed(r *record) bool {
	existing, err := c.ClusterLister.Get("", r.cluster.Name)
	if errors.IsNotFound(err) {
		return true
	} else if err != nil {
		return false
	}

	if existing.Status.APIEndpoint != r.cluster.Status.APIEndpoint ||
		existing.Status.ServiceAccountToken != r.cluster.Status.ServiceAccountToken ||
		existing.Status.CACert != r.cluster.Status.CACert {
		return true
	}

	return false
}

func (c *Manager) watch(r *record) {
	for {
		time.Sleep(15 * time.Second)
		if c.changed(r) {
			c.stop(r)
			break
		}
	}
}

func (c *Manager) stop(r *record) {
	c.servers.Delete(r.cluster.UID)
	go func() {
		time.Sleep(time.Minute)
		r.cancel()
	}()
}

func (c *Manager) toRESTConfig(publicCluster *client.Cluster) (*rest.Config, *v3.Cluster, error) {
	cluster, err := c.ClusterLister.Get("", publicCluster.ID)
	if err != nil {
		return nil, nil, err
	}

	if cluster == nil {
		return nil, nil, nil
	}

	if cluster.Spec.Internal {
		return c.LocalConfig, cluster, nil
	}

	if cluster.Status.APIEndpoint == "" || cluster.Status.CACert == "" || cluster.Status.ServiceAccountToken == "" {
		return nil, nil, nil
	}

	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return nil, nil, err
	}

	caBytes, err := base64.StdEncoding.DecodeString(cluster.Status.CACert)
	if err != nil {
		return nil, nil, err
	}

	return &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: cluster.Status.ServiceAccountToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caBytes,
		},
	}, cluster, nil
}

func (c *Manager) toServer(cluster *client.Cluster) (*record, error) {
	kubeConfig, clusterInternal, err := c.toRESTConfig(cluster)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewClusterContext(c.ManagementConfig, *kubeConfig, cluster.ID)
	if err != nil {
		return nil, err
	}

	s := &record{}
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.handler, err = clusterapi.New(s.ctx, clusterContext)
	if err != nil {
		return nil, err
	}

	s.clusterContext = clusterContext
	s.cluster = clusterInternal.DeepCopy()
	return s, nil
}
