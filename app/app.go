package app

import (
	"context"
	"time"

	"fmt"

	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	managementController "github.com/rancher/rancher/pkg/controllers/management"
	"github.com/rancher/rancher/pkg/dialer"
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

func Run(ctx context.Context, kubeConfig rest.Config, cfg *Config) error {
	management, err := config.NewManagementContext(kubeConfig)
	if err != nil {
		return err
	}
	management.LocalConfig = &kubeConfig

	if err := ReadTLSConfig(cfg); err != nil {
		return err
	}

	for {
		_, err := management.K8sClient.Discovery().ServerVersion()
		if err == nil {
			break
		}
		logrus.Infof("Waiting for server to become available: %v", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("startup canceled")
		case <-time.After(2 * time.Second):
		}
	}

	manager := clustermanager.NewManager(management)
	management.AccessControl = manager
	management.ClientGetter = manager

	server, err := server.New(ctx, cfg.HTTPListenPort, cfg.HTTPSListenPort, management)
	if err != nil {
		return err
	}

	dialerFactory, err := dialer.NewFactory(management, server.Tunneler)
	if err != nil {
		return err
	}

	tokens.StartPurgeDaemon(ctx, management)

	managementController.Register(ctx, management, manager, dialerFactory)
	if err := management.Start(ctx); err != nil {
		return err
	}

	if err := addData(management, *cfg); err != nil {
		return err
	}

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

	return addMachineDrivers(management)
}
