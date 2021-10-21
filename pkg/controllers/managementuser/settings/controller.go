package settings

import (
	"context"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/apply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	toCopy = map[string]bool{
		settings.InstallUUID.Name:     true,
		settings.IngressIPDomain.Name: true,
	}
)

type handler struct {
	clusterID string
	apply     apply.Apply
}

func Register(ctx context.Context, userContext *config.UserContext) error {
	apply, err := apply.NewForConfig(&userContext.RESTConfig)
	if err != nil {
		return err
	}

	h := &handler{
		clusterID: userContext.ClusterName,
		apply:     apply.WithDynamicLookup().WithCacheTypes(userContext.Management.Wrangler.Mgmt.Setting()),
	}

	userContext.Management.Management.Settings("").AddHandler(ctx, "copy-settings", h.onChange)
	return nil
}

func (h *handler) onChange(key string, obj *v3.Setting) (runtime.Object, error) {
	if obj == nil || !toCopy[obj.Name] {
		return nil, nil
	}

	err := h.apply.
		WithOwner(obj).
		ApplyObjects(&v3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: obj.Name,
			},
			Value:   obj.Value,
			Default: obj.Default,
		})
	if err != nil {
		return obj, fmt.Errorf("error applying setting object for cluster [%s]: %v", h.clusterID, err)
	}
	return obj, nil
}
