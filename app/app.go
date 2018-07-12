package app

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"path/filepath"
	"strconv"
	"syscall"

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
	"github.com/sirupsen/logrus"
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
	NewDaemon()
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
		logrus.Infof("Rancher startup complete")

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

func getZombiePid(path string) int {
	// Ignore errors
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return 0
	}

	fields := strings.Split(string(content), ") ")
	fields = strings.Split(fields[len(fields)-1], " ")

	if fields[0] == "Z" && fields[1] == "1" {
		i, _ := strconv.Atoi(strings.Split(string(content), " ")[0])
		return i
	}

	return 0
}

func reaper() {
	reap := make([]string, 0, 5)

	for {
		time.Sleep(1 * time.Second)

		for _, zombie := range reap {
			zombiePid := getZombiePid(zombie)
			logrus.Debugf("Reaping PID %s : %d", zombie, zombiePid)
			if zombiePid <= 0 {
				continue
			}

			reaped, err := syscall.Wait4(zombiePid, nil, syscall.WNOHANG, nil)
			if err != nil || reaped <= 0 {
				logrus.Errorf("Failed to reap %d, got %v: %v", zombiePid, reaped, err)
			}
		}

		reap = reap[:0]

		files, err := filepath.Glob("/proc/*/stat")
		if err != nil {
			logrus.Errorf("Failed to read processes : %v", err)
			continue
		}

		for _, file := range files {
			if getZombiePid(file) > 0 {
				reap = append(reap, file)
			}
		}
	}
}

// NewDaemon sets up everything for the daemon to be able to service
// requests from the webserver.
func NewDaemon() {
	// Reap zombies
	go reaper()
}
