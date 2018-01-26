package agent

import (
	"context"
	"encoding/base64"
	"net/url"
	"sync"

	clusterController "github.com/rancher/rancher/pkg/cluster/controller"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type Manager struct {
	ManagementConfig rest.Config
	LocalConfig      *rest.Config
	controllers      sync.Map
}

type record struct {
	clusterRec *v3.Cluster
	cluster    *config.ClusterContext
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewManager(management *config.ManagementContext) *Manager {
	return &Manager{
		ManagementConfig: management.RESTConfig,
		LocalConfig:      management.LocalConfig,
	}
}

func (m *Manager) Stop(cluster *v3.Cluster) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return
	}
	logrus.Info("Stopping cluster agent for", obj.(*record).cluster.ClusterName)
	obj.(*record).cancel()
	m.controllers.Delete(cluster.UID)
}

func (m *Manager) Start(ctx context.Context, cluster *v3.Cluster) error {
	obj, ok := m.controllers.Load(cluster.UID)
	if ok {
		if !m.changed(obj.(*record), cluster) {
			return nil
		}
		m.Stop(obj.(*record).clusterRec)
	}

	controller, err := m.toRecord(ctx, cluster)
	if controller == nil || err != nil {
		return err
	}

	obj, loaded := m.controllers.LoadOrStore(cluster.UID, controller)
	if !loaded {
		go func() {
			if err := m.doStart(obj.(*record)); err != nil {
				m.Stop(cluster)
			}
		}()
	}

	return nil
}

func (m *Manager) changed(r *record, cluster *v3.Cluster) bool {
	existing := r.clusterRec
	if existing.Status.APIEndpoint != cluster.Status.APIEndpoint ||
		existing.Status.ServiceAccountToken != cluster.Status.ServiceAccountToken ||
		existing.Status.CACert != cluster.Status.CACert {
		return true
	}

	return false
}

func (m *Manager) doStart(rec *record) error {
	logrus.Info("Starting cluster agent for", rec.cluster.ClusterName)
	if err := clusterController.Register(rec.ctx, rec.cluster); err != nil {
		return err
	}
	return rec.cluster.Start(rec.ctx)
}

func (m *Manager) toRESTConfig(cluster *v3.Cluster) (*rest.Config, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Spec.Internal {
		return m.LocalConfig, nil
	}

	if cluster.Status.APIEndpoint == "" || cluster.Status.CACert == "" || cluster.Status.ServiceAccountToken == "" {
		return nil, nil
	}

	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return nil, err
	}

	caBytes, err := base64.StdEncoding.DecodeString(cluster.Status.CACert)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: cluster.Status.ServiceAccountToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caBytes,
		},
	}, nil
}

func (m *Manager) toRecord(ctx context.Context, cluster *v3.Cluster) (*record, error) {
	kubeConfig, err := m.toRESTConfig(cluster)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewClusterContext(m.ManagementConfig, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}

	s := &record{
		cluster:    clusterContext,
		clusterRec: cluster,
	}
	s.ctx, s.cancel = context.WithCancel(ctx)

	return s, nil
}
