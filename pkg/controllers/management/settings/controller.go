package settings

import (
	"context"

	apis "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var toCopy = map[string]bool{}

type handler struct {
	cluster v3.ClusterController
}

func init() {
	defaultSettings := settings.DefaultAgentSettings()

	for _, s := range defaultSettings {
		toCopy[s.Name] = true
	}
}

func Register(ctx context.Context, management *config.ManagementContext) {
	h := &handler{
		cluster: management.Management.Clusters("").Controller(),
	}

	management.Management.Settings("").AddHandler(ctx, "copy-settings", h.onChange)
}

func (h *handler) onChange(key string, obj *apis.Setting) (runtime.Object, error) {
	if obj == nil || !toCopy[obj.Name] {
		return nil, nil
	}

	clusters, err := h.cluster.Lister().List("", labels.Everything())
	if err != nil {
		return obj, err
	}
	for _, c := range clusters {
		if c.Name != "local" {
			h.cluster.Enqueue("", c.Name)
		}
	}

	return obj, nil
}
