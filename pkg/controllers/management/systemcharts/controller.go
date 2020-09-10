package systemcharts

import (
	"context"
	"sync"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	toInstall = []chartDef{
		{
			ReleaseNamespace: namespaces.System,
			ChartName:        "rancher-webhook",
			Version:          "~0-a",
		},
		{
			ReleaseNamespace: "rancher-operator-system",
			ChartName:        "rancher-operator-crd",
			Version:          "~0-a",
		},
		{
			ReleaseNamespace: "rancher-operator-system",
			ChartName:        "rancher-operator",
			Version:          "~0-a",
		},
	}
	repoName = "rancher-charts"
)

type chartDef struct {
	ReleaseNamespace string
	ChartName        string
	Version          string
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-charts", h.onRepo)
	return nil
}

type handler struct {
	manager *system.Manager
	once    sync.Once
}

func (h *handler) onRepo(key string, repo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	if repo == nil || repo.Name != repoName {
		return repo, nil
	}

	h.once.Do(func() {
		go func() {
			b := wait.Backoff{Factor: 1.5, Duration: time.Second, Steps: 100, Cap: 2 * time.Minute}
			for {
				if err := h.installCharts(); err != nil {
					logrus.Errorf("Failed to bootstrap charts, waiting 2 seconds: %v", err)
					time.Sleep(b.Step())
					continue
				}
				break
			}
		}()
	})

	return repo, nil
}

func (h *handler) installCharts() error {
	for _, chartDef := range toInstall {
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, chartDef.Version, nil); err != nil {
			return err
		}
	}

	return nil
}
