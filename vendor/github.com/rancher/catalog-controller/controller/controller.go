package controller

import (
	"os"
	"path"

	"context"

	"time"

	"github.com/rancher/catalog-controller/manager"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	// TODO: Get values from settings
	if err := Run(ctx, "", 3600, management); err != nil {
		panic(err)
	}
}

func runRefresh(ctx context.Context, interval int, controller v3.CatalogController, m *manager.Manager) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			catalogs, err := m.GetCatalogs()
			if err != nil {
				logrus.Error(err)
				continue
			}
			for _, catalog := range catalogs {
				controller.Enqueue("", catalog.Name)
			}
		}
	}
}

func Run(ctx context.Context, cacheRoot string, refreshInterval int, management *config.ManagementContext) error {
	if cacheRoot == "" {
		cacheRoot = path.Join(os.Getenv("HOME"), ".catalog-controller", "cache")
	}

	logrus.Infof("Starting catalog controller")
	m := manager.New(management, cacheRoot)

	controller := management.Management.Catalogs("").Controller()
	controller.AddHandler("catalog", m.Sync)

	go runRefresh(ctx, refreshInterval, controller, m)

	return nil
}
