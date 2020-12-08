package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/multiclustermanager/clustermanager"
	managementController "github.com/rancher/rancher/pkg/multiclustermanager/controllers/management"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/eksupstreamrefresh"
	managementcrds "github.com/rancher/rancher/pkg/multiclustermanager/crds/management"
	"github.com/rancher/rancher/pkg/multiclustermanager/cron"
	managementdata "github.com/rancher/rancher/pkg/multiclustermanager/data/management"
	"github.com/rancher/rancher/pkg/multiclustermanager/dialer"
	"github.com/rancher/rancher/pkg/multiclustermanager/jailer"
	"github.com/rancher/rancher/pkg/multiclustermanager/metrics"
	"github.com/rancher/rancher/pkg/multiclustermanager/options"
	"github.com/rancher/rancher/pkg/multiclustermanager/tunnelserver"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type mcm struct {
	ScaledContext       *config.ScaledContext
	clusterManager      *clustermanager.Manager
	router              func(http.Handler) http.Handler
	wranglerContext     *wrangler.Context
	localClusterEnabled bool
	removeLocalCluster  bool
	embedded            bool
	httpsListenPort     int

	startedChan chan struct{}
	startLock   sync.Mutex
}

func buildScaledContext(ctx context.Context, wranglerContext *wrangler.Context, cfg *options.Options) (*config.ScaledContext, *clustermanager.Manager, error) {
	scaledContext, err := config.NewScaledContext(*wranglerContext.RESTConfig, &config.ScaleContextOptions{
		ControllerFactory: wranglerContext.ControllerFactory,
	})
	if err != nil {
		return nil, nil, err
	}

	if err := managementcrds.Create(ctx, wranglerContext.RESTConfig); err != nil {
		return nil, nil, err
	}

	dialerFactory, err := dialer.NewFactory(scaledContext)
	if err != nil {
		return nil, nil, err
	}

	scaledContext.Dialer = dialerFactory
	scaledContext.PeerManager, err = tunnelserver.NewPeerManager(ctx, scaledContext, dialerFactory.TunnelServer)
	if err != nil {
		return nil, nil, err
	}

	userManager, err := common.NewUserManager(scaledContext)
	if err != nil {
		return nil, nil, err
	}

	scaledContext.UserManager = userManager
	scaledContext.RunContext = ctx

	manager := clustermanager.NewManager(cfg.HTTPSListenPort, scaledContext, wranglerContext.RBAC, wranglerContext.ASL)

	scaledContext.AccessControl = manager
	scaledContext.ClientGetter = manager

	return scaledContext, manager, nil
}

func NewMCM(ctx context.Context, wranglerContext *wrangler.Context, cfg *options.Options) (wrangler.MultiClusterManager, error) {
	scaledContext, clusterManager, err := buildScaledContext(ctx, wranglerContext, cfg)
	if err != nil {
		return nil, err
	}

	router, err := router(ctx, cfg.LocalClusterEnabled, scaledContext, clusterManager)
	if err != nil {
		return nil, err
	}

	if os.Getenv("CATTLE_PROMETHEUS_METRICS") == "true" {
		metrics.Register(ctx, scaledContext)
	}

	mcm := &mcm{
		router:              router,
		ScaledContext:       scaledContext,
		clusterManager:      clusterManager,
		wranglerContext:     wranglerContext,
		localClusterEnabled: cfg.LocalClusterEnabled,
		removeLocalCluster:  cfg.RemoveLocalCluster,
		embedded:            cfg.Embedded,
		httpsListenPort:     cfg.HTTPSListenPort,
		startedChan:         make(chan struct{}),
	}

	go func() {
		<-ctx.Done()
		mcm.started()
	}()

	return mcm, nil
}

func (m *mcm) ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer, err := m.ScaledContext.Dialer.ClusterDialer(clusterID)
		if err != nil {
			return nil, err
		}
		return dialer(ctx, network, address)
	}
}

func (m *mcm) K8sClient(clusterName string) (kubernetes.Interface, error) {
	return m.K8sClient(clusterName)
}

func (m *mcm) started() {
	m.startLock.Lock()
	defer m.startLock.Unlock()
	select {
	case <-m.startedChan:
	default:
		close(m.startedChan)
	}
}

func (m *mcm) Wait(ctx context.Context) {
	select {
	case <-m.startedChan:
		for {
			if _, err := m.wranglerContext.Core.Namespace().Get(namespace.GlobalNamespace, metav1.GetOptions{}); err == nil {
				return
			}
			logrus.Infof("Waiting for initial data to be populated")
			time.Sleep(2 * time.Second)
		}
	case <-ctx.Done():
	}
}

func (m *mcm) Middleware(next http.Handler) http.Handler {
	return m.router(next)
}

func (m *mcm) Start(ctx context.Context) error {
	var (
		management *config.ManagementContext
	)

	defer m.started()

	if dm := os.Getenv("CATTLE_DEV_MODE"); dm == "" {
		if err := jailer.CreateJail("driver-jail"); err != nil {
			return err
		}

		if err := cron.StartJailSyncCron(m.ScaledContext); err != nil {
			return err
		}
	}

	m.wranglerContext.OnLeader(func(ctx context.Context) error {
		err := m.wranglerContext.StartWithTransaction(ctx, func(ctx context.Context) error {
			var (
				err error
			)

			if m.ScaledContext.PeerManager != nil {
				m.ScaledContext.PeerManager.Leader()
			}

			management, err = m.ScaledContext.NewManagementContext()
			if err != nil {
				return errors.Wrap(err, "failed to create management context")
			}

			if err := managementdata.Add(m.wranglerContext, management); err != nil {
				return errors.Wrap(err, "failed to add management data")
			}

			managementController.Register(ctx, management, m.ScaledContext.ClientGetter.(*clustermanager.Manager))
			if err := managementController.RegisterWrangler(ctx, m.wranglerContext, management, m.ScaledContext.ClientGetter.(*clustermanager.Manager)); err != nil {
				return errors.Wrap(err, "failed to register wrangler controllers")
			}
			return nil
		})
		if err != nil {
			return err
		}

		if err := telemetry.Start(ctx, m.httpsListenPort, m.ScaledContext); err != nil {
			return errors.Wrap(err, "failed to telemetry")
		}

		tokens.StartPurgeDaemon(ctx, management)
		providerrefresh.StartRefreshDaemon(ctx, m.ScaledContext, management)
		managementdata.CleanupOrphanedSystemUsers(ctx, management)
		eksupstreamrefresh.StartEKSUpstreamCronJob(m.wranglerContext)
		logrus.Infof("Rancher startup complete")
		return nil
	})

	return nil
}
