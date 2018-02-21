package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/norman/leader"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	managementController "github.com/rancher/rancher/pkg/controllers/management"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rancher/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type Config struct {
	HTTPOnly          bool
	ACMEDomains       []string
	KubeConfig        string
	HTTPListenPort    int
	HTTPSListenPort   int
	InteralListenPort int
	K8sMode           string
	AddLocal          bool
	Debug             bool
	ListenConfig      *v3.ListenConfig
}

var defaultAdminLabel = map[string]string{"authz.management.cattle.io/bootstrapping": "admin-user"}

func buildManagement(scaledContext *config.ScaledContext) (*config.ManagementContext, error) {
	management, err := config.NewManagementContext(scaledContext.RESTConfig)
	if err != nil {
		return nil, err
	}
	management.Dialer = scaledContext.Dialer
	return management, nil
}

func buildScaledContext(ctx context.Context, kubeConfig rest.Config, cfg *Config) (*config.ScaledContext, error) {
	scaledContext, err := config.NewScaledContext(kubeConfig)
	if err != nil {
		return nil, err
	}
	scaledContext.LocalConfig = &kubeConfig

	if err := ReadTLSConfig(cfg); err != nil {
		return nil, err
	}

	for {
		_, err := scaledContext.K8sClient.Discovery().ServerVersion()
		if err == nil {
			break
		}
		logrus.Infof("Waiting for server to become available: %v", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("startup canceled")
		case <-time.After(2 * time.Second):
		}
	}

	tunnelServer := tunnelserver.NewTunnelServer(scaledContext)
	dialerFactory, err := dialer.NewFactory(scaledContext, tunnelServer)
	if err != nil {
		return nil, err
	}

	scaledContext.Dialer = dialerFactory

	manager := clustermanager.NewManager(scaledContext)
	scaledContext.AccessControl = manager
	scaledContext.ClientGetter = manager

	return scaledContext, nil
}

func Run(ctx context.Context, kubeConfig rest.Config, cfg *Config) error {
	scaledContext, err := buildScaledContext(ctx, kubeConfig, cfg)
	if err != nil {
		return err
	}

	if err := server.Start(ctx, cfg.HTTPListenPort, cfg.HTTPSListenPort, scaledContext); err != nil {
		return err
	}

	if err := scaledContext.Start(ctx); err != nil {
		return err
	}

	go leader.RunOrDie(ctx, "cattle-controllers", scaledContext.K8sClient, func(ctx context.Context) {
		scaledContext.Leader = true

		management, err := buildManagement(scaledContext)
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

	if err := addRoles(management, cfg.AddLocal); err != nil {
		return err
	}

	if err := addAuthConfigs(management); err != nil {
		return err
	}

	if err := addCatalogs(management); err != nil {
		return err
	}

	return addMachineDrivers(management)
}
