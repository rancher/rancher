package app

import (
	"context"
	"os"

	"github.com/rancher/norman/pkg/k8scheck"
	"github.com/rancher/rancher/pkg/audit"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	managementController "github.com/rancher/rancher/pkg/controllers/management"
	"github.com/rancher/rancher/pkg/cron"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rancher/server"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type Config struct {
	ACMEDomains       []string
	AddLocal          string
	Embedded          bool
	KubeConfig        string
	HTTPListenPort    int
	HTTPSListenPort   int
	K8sMode           string
	Debug             bool
	NoCACerts         bool
	ListenConfig      *v3.ListenConfig
	AuditLogPath      string
	AuditLogMaxage    int
	AuditLogMaxsize   int
	AuditLogMaxbackup int
	AuditLevel        int
	Features          string
}

func buildScaledContext(ctx context.Context, kubeConfig rest.Config, cfg *Config) (*config.ScaledContext, *clustermanager.Manager, error) {
	scaledContext, err := config.NewScaledContext(kubeConfig)
	if err != nil {
		return nil, nil, err
	}
	scaledContext.LocalConfig = &kubeConfig

	cfg.ListenConfig, err = tls.ReadTLSConfig(cfg.ACMEDomains, cfg.NoCACerts)
	if err != nil {
		return nil, nil, err
	}

	if err := k8scheck.Wait(ctx, kubeConfig); err != nil {
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

	manager := clustermanager.NewManager(cfg.HTTPSListenPort, scaledContext)
	scaledContext.AccessControl = manager
	scaledContext.ClientGetter = manager

	userManager, err := common.NewUserManager(scaledContext)
	if err != nil {
		return nil, nil, err
	}

	scaledContext.UserManager = userManager

	return scaledContext, manager, nil
}

func Run(ctx context.Context, kubeConfig rest.Config, cfg *Config) error {
	scaledContext, clusterManager, err := buildScaledContext(ctx, kubeConfig, cfg)
	if err != nil {
		return err
	}

	auditLogWriter := audit.NewLogWriter(cfg.AuditLogPath, cfg.AuditLevel, cfg.AuditLogMaxage, cfg.AuditLogMaxbackup, cfg.AuditLogMaxsize)

	if err := server.Start(ctx, cfg.HTTPListenPort, cfg.HTTPSListenPort, localClusterEnabled(*cfg), scaledContext, clusterManager, auditLogWriter); err != nil {
		return err
	}

	// this step needs to happen prior to starting scaled context to ensure
	// cache works properly
	features.InitializeFeatures(scaledContext, cfg.Features)

	if os.Getenv("CATTLE_PROMETHEUS_METRICS") != "true" {
		metrics.Register(ctx, scaledContext)
	}

	if err := scaledContext.Start(ctx); err != nil {
		return err
	}

	if dm := os.Getenv("CATTLE_DEV_MODE"); dm == "" {
		if err := jailer.CreateJail("driver-jail"); err != nil {
			return err
		}

		if err := cron.StartJailSyncCron(scaledContext); err != nil {
			return err
		}
	}

	go leader.RunOrDie(ctx, "", "cattle-controllers", scaledContext.K8sClient, func(ctx context.Context) {
		if scaledContext.PeerManager != nil {
			scaledContext.PeerManager.Leader()
		}

		management, err := scaledContext.NewManagementContext()
		if err != nil {
			panic(err)
		}

		managementController.Register(ctx, management, scaledContext.ClientGetter.(*clustermanager.Manager))
		if err := management.Start(ctx); err != nil {
			panic(err)
		}

		if err := addData(management, *cfg); err != nil {
			panic(err)
		}

		if err := telemetry.Start(ctx, cfg.HTTPSListenPort, scaledContext); err != nil {
			panic(err)
		}

		tokens.StartPurgeDaemon(ctx, management)
		cronTime := settings.AuthUserInfoResyncCron.Get()
		maxAge := settings.AuthUserInfoMaxAgeSeconds.Get()
		providerrefresh.StartRefreshDaemon(ctx, scaledContext, management, cronTime, maxAge)
		cleanupOrphanedSystemUsers(ctx, management)
		logrus.Infof("Rancher startup complete")

		<-ctx.Done()
	})

	<-ctx.Done()

	if auditLogWriter != nil {
		auditLogWriter.Output.Close()
	}
	return ctx.Err()
}

func addData(management *config.ManagementContext, cfg Config) error {
	if err := addListenConfig(management, cfg); err != nil {
		return err
	}

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

	if err := addAuthConfigs(management); err != nil {
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
