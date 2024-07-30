package common

import (
	"context"
	"testing"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func RegisterCRDs(ctx context.Context, t *testing.T, r *rest.Config, crds ...crd.CRD) {
	factory, err := crd.NewFactoryFromClient(r)
	assert.NoError(t, err)

	err = factory.BatchCreateCRDs(ctx, crds...).BatchWait()
	assert.NoError(t, err)
}

func StartNormanControllers(ctx context.Context, t *testing.T, m *config.ManagementContext, gvk ...schema.GroupVersionKind) {
	controllers := []controller.SharedController{}
	for _, g := range gvk {
		c, err := m.ControllerFactory.ForKind(g)
		assert.NoError(t, err)
		controllers = append(controllers, c)
	}
	for _, c := range controllers {
		err := c.Start(ctx, 1)
		assert.NoError(t, err)
	}
}

func StartWranglerCaches(ctx context.Context, t *testing.T, w *wrangler.Context, gvk ...schema.GroupVersionKind) {
	for _, g := range gvk {
		err := w.ControllerFactory.SharedCacheFactory().StartGVK(ctx, g)
		assert.NoError(t, err)
	}
}
