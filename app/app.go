package app

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/rancher/norman/pkg/k8scheck"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/data"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	managementController "github.com/rancher/rancher/pkg/controllers/management"
	"github.com/rancher/rancher/pkg/crds"
	"github.com/rancher/rancher/pkg/cron"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/steve"
	"github.com/rancher/rancher/pkg/steve/pkg/clusterapi"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/server"
	"github.com/rancher/remotedialer"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/auth"
	steveserver "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	ACMEDomains       cli.StringSlice
	AddLocal          string
	Embedded          bool
	HTTPListenPort    int
	HTTPSListenPort   int
	K8sMode           string
	Debug             bool
	Trace             bool
	NoCACerts         bool
	AuditLogPath      string
	AuditLogMaxage    int
	AuditLogMaxsize   int
	AuditLogMaxbackup int
	AuditLevel        int
	Features          string
}

type Rancher struct {
	Config          Config
	AccessSetLookup accesscontrol.AccessSetLookup
	Handler         http.Handler
	Auth            auth.Middleware
	ScaledContext   *config.ScaledContext
	WranglerContext *wrangler.Context
	ClusterManager  *clustermanager.Manager
}

func (r *Rancher) ListenAndServe(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}

	if err := tls.ListenAndServe(ctx, &r.ScaledContext.RESTConfig,
		r.Handler,
		r.Config.HTTPSListenPort,
		r.Config.HTTPListenPort,
		r.Config.ACMEDomains,
		r.Config.NoCACerts); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

func initFeatures(ctx context.Context, scaledContext *config.ScaledContext, cfg *Config) error {
	factory, err := crd.NewFactoryFromClient(&scaledContext.RESTConfig)
	if err != nil {
		return err
	}

	if err := crds.Create(ctx, &scaledContext.RESTConfig); err != nil {
		return err
	}

	if _, err := factory.CreateCRDs(ctx, crd.NonNamespacedType("Feature.management.cattle.io/v3")); err != nil {
		return err
	}

	scaledContext.Management.Features("").Controller()
	if err := scaledContext.Start(ctx); err != nil {
		return err
	}
	features.InitializeFeatures(scaledContext, cfg.Features)
	return nil
}

func buildScaledContext(ctx context.Context, clientConfig clientcmd.ClientConfig, cfg *Config) (*config.ScaledContext, *clustermanager.Manager, *wrangler.Context, error) {
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	restConfig.Timeout = 30 * time.Second
	restConfig.RateLimiter = ratelimit.None

	kubeConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := k8scheck.Wait(ctx, *restConfig); err != nil {
		return nil, nil, nil, err
	}

	scaledContext, err := config.NewScaledContext(*restConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	scaledContext.KubeConfig = kubeConfig

	if err := initFeatures(ctx, scaledContext, cfg); err != nil {
		return nil, nil, nil, err
	}

	dialerFactory, err := dialer.NewFactory(scaledContext)
	if err != nil {
		return nil, nil, nil, err
	}

	scaledContext.Dialer = dialerFactory
	scaledContext.PeerManager, err = tunnelserver.NewPeerManager(ctx, scaledContext, dialerFactory.TunnelServer)
	if err != nil {
		return nil, nil, nil, err
	}

	var tunnelServer *remotedialer.Server
	if df, ok := scaledContext.Dialer.(*dialer.Factory); ok {
		tunnelServer = df.TunnelServer
	}

	wranglerContext, err := wrangler.NewContext(ctx, steveserver.RestConfigDefaults(&scaledContext.RESTConfig), scaledContext.ControllerFactory, tunnelServer)
	if err != nil {
		return nil, nil, nil, err
	}

	userManager, err := common.NewUserManager(scaledContext)
	if err != nil {
		return nil, nil, nil, err
	}

	scaledContext.UserManager = userManager
	scaledContext.RunContext = ctx

	manager := clustermanager.NewManager(cfg.HTTPSListenPort, scaledContext, wranglerContext.RBAC, wranglerContext.ASL)
	scaledContext.AccessControl = manager
	scaledContext.ClientGetter = manager

	return scaledContext, manager, wranglerContext, nil
}

func New(ctx context.Context, clientConfig clientcmd.ClientConfig, cfg *Config) (*Rancher, error) {
	scaledContext, clusterManager, wranglerContext, err := buildScaledContext(ctx, clientConfig, cfg)
	if err != nil {
		return nil, err
	}

	auditLogWriter := audit.NewLogWriter(cfg.AuditLogPath, cfg.AuditLevel, cfg.AuditLogMaxage, cfg.AuditLogMaxbackup, cfg.AuditLogMaxsize)

	authMiddleware, handler, err := server.Start(ctx, localClusterEnabled(*cfg), scaledContext, clusterManager, auditLogWriter, rbac.NewAccessControlHandler())
	if err != nil {
		return nil, err
	}

	if os.Getenv("CATTLE_PROMETHEUS_METRICS") == "true" {
		metrics.Register(ctx, scaledContext)
	}

	if auditLogWriter != nil {
		go func() {
			<-ctx.Done()
			auditLogWriter.Output.Close()
		}()
	}

	rancher := &Rancher{
		AccessSetLookup: wranglerContext.ASL,
		Config:          *cfg,
		Handler:         handler,
		Auth:            authMiddleware,
		ScaledContext:   scaledContext,
		WranglerContext: wranglerContext,
		ClusterManager:  clusterManager,
	}

	return rancher, nil
}

func (r *Rancher) Start(ctx context.Context) error {
	if err := r.ScaledContext.Start(ctx); err != nil {
		return err
	}

	if err := r.WranglerContext.Start(ctx); err != nil {
		return err
	}

	if dm := os.Getenv("CATTLE_DEV_MODE"); dm == "" {
		if err := jailer.CreateJail("driver-jail"); err != nil {
			return err
		}

		if err := cron.StartJailSyncCron(r.ScaledContext); err != nil {
			return err
		}
	}

	go leader.RunOrDie(ctx, "", "cattle-controllers", r.ScaledContext.K8sClient, func(ctx context.Context) {
		if r.ScaledContext.PeerManager != nil {
			r.ScaledContext.PeerManager.Leader()
		}

		management, err := r.ScaledContext.NewManagementContext()
		if err != nil {
			panic(err)
		}

		managementController.Register(ctx, management, r.ScaledContext.ClientGetter.(*clustermanager.Manager))
		if err := management.Start(ctx); err != nil {
			panic(err)
		}

		if err := managementController.RegisterWrangler(ctx, r.WranglerContext, management, r.ScaledContext.ClientGetter.(*clustermanager.Manager)); err != nil {
			panic(err)
		}

		if err := r.WranglerContext.Start(ctx); err != nil {
			panic(err)
		}

		if err := addData(management, r.Config); err != nil {
			panic(err)
		}

		if err := telemetry.Start(ctx, r.Config.HTTPSListenPort, r.ScaledContext); err != nil {
			panic(err)
		}

		tokens.StartPurgeDaemon(ctx, management)
		providerrefresh.StartRefreshDaemon(ctx, r.ScaledContext, management)
		cleanupOrphanedSystemUsers(ctx, management)
		logrus.Infof("Rancher startup complete")

		<-ctx.Done()
	})

	if features.Steve.Enabled() {
		handler, err := newSteve(ctx, r)
		if err != nil {
			return err
		}
		r.Handler = handler
	}

	return nil
}

func addData(management *config.ManagementContext, cfg Config) error {
	adminName, err := addRoles(management)
	if err != nil {
		return err
	}

	if localClusterEnabled(cfg) {
		if err := addLocalCluster(cfg.Embedded, adminName, management); err != nil {
			return err
		}
	} else if cfg.AddLocal == "false" {
		if err := removeLocalCluster(management); err != nil {
			return err
		}
	}

	if err := data.AuthConfigs(management); err != nil {
		return err
	}

	if err := syncCatalogs(management); err != nil {
		return err
	}

	if err := addSetting(); err != nil {
		return err
	}

	if err := addDefaultPodSecurityPolicyTemplates(management); err != nil {
		return err
	}

	if err := addKontainerDrivers(management); err != nil {
		return err
	}

	if err := addCattleGlobalNamespaces(management); err != nil {
		return err
	}

	return addMachineDrivers(management)
}

func localClusterEnabled(cfg Config) bool {
	if cfg.AddLocal == "true" || (cfg.AddLocal == "auto" && !cfg.Embedded) {
		return true
	}
	return false
}

func newSteve(ctx context.Context, rancher *Rancher) (http.Handler, error) {
	clusterapiServer := &clusterapi.Server{}
	cfg := steveserver.Server{
		AccessSetLookup: rancher.AccessSetLookup,
		Controllers:     rancher.WranglerContext.Controllers,
		RESTConfig:      steveserver.RestConfigDefaults(&rancher.ScaledContext.RESTConfig),
		AuthMiddleware:  rancher.Auth,
		Next:            rancher.Handler,
		StartHooks: []steveserver.StartHook{
			func(ctx context.Context, server *steveserver.Server) error {
				return steve.Setup(server, rancher.WranglerContext, localClusterEnabled(rancher.Config), rancher.Handler)
			},
			clusterapiServer.Setup,
		},
	}

	localSteve, err := cfg.Handler(ctx)
	if err != nil {
		return nil, err
	}

	return clusterapiServer.Wrap(localSteve), nil
}
