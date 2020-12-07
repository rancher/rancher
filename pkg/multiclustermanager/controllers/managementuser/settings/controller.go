package settings

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/apply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	toCopy = map[string]bool{
		settings.SystemDefaultRegistry.Name: true,
		settings.InstallUUID.Name:           true,
	}
)

type handler struct {
	apply apply.Apply
}

func Register(ctx context.Context, userContext *config.UserContext) error {
	apply, err := apply.NewForConfig(&userContext.RESTConfig)
	if err != nil {
		return err
	}

	h := &handler{
		apply: apply.WithDynamicLookup(),
	}

	userContext.Management.Management.Settings("").AddHandler(ctx, "copy-settings", h.onChange)
	return nil
}

func (h *handler) onChange(key string, obj *v3.Setting) (runtime.Object, error) {
	if obj == nil || !toCopy[obj.Name] {
		return nil, nil
	}

	return obj, h.apply.
		WithOwner(obj).
		ApplyObjects(&v3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: obj.Name,
			},
			Value:   obj.Value,
			Default: obj.Default,
		})
}
