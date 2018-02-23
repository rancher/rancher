package clustermanager

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	clusterController "github.com/rancher/rancher/pkg/controllers/user"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"
)

type Manager struct {
	APIContext    *config.ScaledContext
	clusterLister v3.ClusterLister
	controllers   sync.Map
	accessControl types.AccessControl
	dialer        dialer.Factory
}

type record struct {
	clusterRec    *v3.Cluster
	cluster       *config.UserContext
	accessControl types.AccessControl
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewManager(context *config.ScaledContext) *Manager {
	return &Manager{
		APIContext:    context,
		dialer:        context.Dialer,
		accessControl: rbac.NewAccessControl(context.RBAC),
		clusterLister: context.Management.Clusters("").Controller().Lister(),
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
	_, err := m.start(ctx, cluster)
	return err
}

func (m *Manager) RESTConfig(ctx context.Context, cluster *v3.Cluster) (rest.Config, error) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return rest.Config{}, fmt.Errorf("cluster record not found %s %s", cluster.Name, cluster.UID)
	}

	record := obj.(*record)
	return record.cluster.RESTConfig, nil
}

func (m *Manager) start(ctx context.Context, cluster *v3.Cluster) (*record, error) {
	obj, ok := m.controllers.Load(cluster.UID)
	if ok {
		if !m.changed(obj.(*record), cluster) {
			return obj.(*record), nil
		}
		m.Stop(obj.(*record).clusterRec)
	}

	controller, err := m.toRecord(ctx, cluster)
	if controller == nil || err != nil {
		return nil, err
	}

	obj, loaded := m.controllers.LoadOrStore(cluster.UID, controller)
	if !loaded {
		go func() {
			if err := m.doStart(obj.(*record)); err != nil {
				m.Stop(cluster)
			}
		}()
	}

	return obj.(*record), nil
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

	if cluster.Status.Driver == v3.ClusterDriverLocal {
		return m.APIContext.LocalConfig, nil
	}

	if cluster.Status.APIEndpoint == "" || cluster.Status.CACert == "" || cluster.Status.ServiceAccountToken == "" {
		return nil, nil
	}

	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
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

	dialer, err := m.dialer.ClusterDialer(cluster.Name)
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
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			if ht, ok := rt.(*http.Transport); ok {
				ht.DialContext = nil
				ht.DialTLS = nil
				ht.Dial = dialer
			}
			return rt
		},
	}, nil
}

func (m *Manager) toRecord(ctx context.Context, cluster *v3.Cluster) (*record, error) {
	kubeConfig, err := m.toRESTConfig(cluster)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewUserContext(m.APIContext.RESTConfig, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}

	s := &record{
		cluster:       clusterContext,
		clusterRec:    cluster,
		accessControl: rbac.NewAccessControl(clusterContext.RBAC),
	}
	s.ctx, s.cancel = context.WithCancel(ctx)

	return s, nil
}

func (m *Manager) AccessControl(apiContext *types.APIContext, storageContext types.StorageContext) (types.AccessControl, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.accessControl, nil
	}
	return record.accessControl, nil
}

func (m *Manager) Config(apiContext *types.APIContext, storageContext types.StorageContext) (rest.Config, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return rest.Config{}, err
	}
	if record == nil {
		return m.APIContext.RESTConfig, nil
	}
	return record.cluster.RESTConfig, nil
}

func (m *Manager) UnversionedClient(apiContext *types.APIContext, storageContext types.StorageContext) (rest.Interface, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.APIContext.UnversionedClient, nil
	}
	return record.cluster.UnversionedClient, nil
}

func (m *Manager) APIExtClient(apiContext *types.APIContext, storageContext types.StorageContext) (clientset.Interface, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.APIContext.APIExtClient, nil
	}
	return record.cluster.APIExtClient, nil
}

func (m *Manager) record(apiContext *types.APIContext, storageContext types.StorageContext) (*record, error) {
	if apiContext == nil {
		return nil, nil
	}
	cluster, err := m.cluster(apiContext, storageContext)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}
	if cluster == nil {
		return nil, nil
	}
	record, err := m.start(context.Background(), cluster)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}

	return record, nil
}

func (m *Manager) cluster(apiContext *types.APIContext, context types.StorageContext) (*v3.Cluster, error) {
	switch context {
	case types.DefaultStorageContext:
		return nil, nil
	case config.ManagementStorageContext:
		return nil, nil
	case config.UserStorageContext:
	default:
		return nil, fmt.Errorf("illegal context: %s", context)

	}

	clusterID := apiContext.SubContext["/v3/schemas/cluster"]
	if clusterID == "" {
		projectID, ok := apiContext.SubContext["/v3/schemas/project"]
		if ok {
			parts := strings.SplitN(projectID, ":", 2)
			if len(parts) == 2 {
				clusterID = parts[0]
			}
		}
	}

	if clusterID == "" {
		return nil, fmt.Errorf("failed to find cluster ID")
	}

	return m.clusterLister.Get("", clusterID)
}
