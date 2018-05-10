package app

import (
	"context"

	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/norman/leader"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	managementController "github.com/rancher/rancher/pkg/controllers/management"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/k8scheck"
	"github.com/rancher/rancher/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/client-go/rest"
)

type Config struct {
	ACMEDomains     []string
	AddLocal        string
	Embedded        bool
	KubeConfig      string
	HTTPListenPort  int
	HTTPSListenPort int
	K8sMode         string
	Debug           bool
	NoCACerts       bool
	ListenConfig    *v3.ListenConfig
}

func buildScaledContext(ctx context.Context, kubeConfig rest.Config, cfg *Config) (*config.ScaledContext, *clustermanager.Manager, error) {
	scaledContext, err := config.NewScaledContext(kubeConfig)
	if err != nil {
		return nil, nil, err
	}
	scaledContext.LocalConfig = &kubeConfig

	if err := ReadTLSConfig(cfg); err != nil {
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
	if err := service.Start(); err != nil {
		return err
	}

	scaledContext, clusterManager, err := buildScaledContext(ctx, kubeConfig, cfg)
	if err != nil {
		return err
	}

	if err := server.Start(ctx, cfg.HTTPListenPort, cfg.HTTPSListenPort, scaledContext, clusterManager); err != nil {
		return err
	}

	if err := scaledContext.Start(ctx); err != nil {
		return err
	}

	go leader.RunOrDie(ctx, "cattle-controllers", scaledContext.K8sClient, func(ctx context.Context) {
		scaledContext.Leader = true

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

		tokens.StartPurgeDaemon(ctx, management)

		<-ctx.Done()
	})

	<-ctx.Done()
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

	if cfg.AddLocal == "true" || (cfg.AddLocal == "auto" && !cfg.Embedded) {
		if err := addLocalCluster(cfg.Embedded, adminName, management); err != nil {
			return err
		}
	}

	if err := addAuthConfigs(management); err != nil {
		return err
	}

	if err := addCatalogs(management); err != nil {
		return err
	}

	if err := addSetting(); err != nil {
		return err
	}

	return addMachineDrivers(management)
}
