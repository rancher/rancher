package multiclustermanager

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/clustermanager"
	managementController "github.com/rancher/rancher/pkg/controllers/management"
	"github.com/rancher/rancher/pkg/controllers/management/eksupstreamrefresh"
	managementcrds "github.com/rancher/rancher/pkg/crds/management"
	"github.com/rancher/rancher/pkg/cron"
	managementdata "github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	LocalClusterEnabled bool
	RemoveLocalCluster  bool
	Embedded            bool
	HTTPSListenPort     int
	Debug               bool
	Trace               bool
}

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

func buildScaledContext(ctx context.Context, wranglerContext *wrangler.Context, cfg *Options) (*config.ScaledContext, *clustermanager.Manager, error) {
	scaledContext, err := config.NewScaledContext(*wranglerContext.RESTConfig, &config.ScaleContextOptions{
		ControllerFactory: wranglerContext.ControllerFactory,
	})
	if err != nil {
		return nil, nil, err
	}

	scaledContext.CatalogManager = manager.New(scaledContext.Management, scaledContext.Project)

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

func newMCM(ctx context.Context, wranglerContext *wrangler.Context, cfg *Options) (*mcm, error) {
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

			if err := managementdata.Add(m.wranglerContext, management, m.localClusterEnabled, m.removeLocalCluster, m.embedded); err != nil {
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
