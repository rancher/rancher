package wrangler

import (
	"context"
	"sync/atomic"

	ext "github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io"
	extv1 "github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io/v1"
	wapiregv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

// EXTAPIContext is a scoped context which wraps the larger Wrangler context.
// It includes EXT clients and factories which are initialized after the EXT
// api-service is detected and established
type EXTAPIContext struct {
	*Context
	Ext extv1.Interface
	ext *ext.Factory
}

// DeferredEXTAPIInitializer implements the DeferredInitializer interface and
// monitors api-services until the expected EXT api-service resource is created
// and established.
type DeferredEXTAPIInitializer struct {
	*BaseInitializer[*EXTAPIContext]
}

func NewEXTAPIInitializer() *DeferredEXTAPIInitializer {
	return &DeferredEXTAPIInitializer{
		BaseInitializer: NewBaseInitializer[*EXTAPIContext](),
	}
}

// OnChange is the initializer method spawning a waiter for the availability of
// the EXT api-service. On success it initializes the context, enabling the
// controlling manager to run the associated deferred functions.
func (d *DeferredEXTAPIInitializer) OnChange(ctx context.Context, c *Context) {
	logrus.Info("[deferred-ext] OnChange starting waiter for EXT api-service availability")
	var done atomic.Bool
	c.API.APIService().OnChange(ctx, "extapi-deferred-registration", func(key string, api *apiregv1.APIService) (*apiregv1.APIService, error) {
		if done.Load() {
			return api, nil
		}

		if !extReady(c, c.API.APIService().Cache()) {
			return api, nil
		}

		if !done.CompareAndSwap(false, true) {
			return api, nil
		}

		logrus.Debugf("[deferred-ext] %p ext factory create", c)

		ext, err := ext.NewFactoryFromConfigWithOptions(c.RESTConfig, &generic.FactoryOptions{
			SharedControllerFactory: c.ControllerFactory,
		})
		if err != nil {
			logrus.Fatalf("Encountered unexpected error while creating ext factory: %v", err)
		}

		d.SetClientContext(&EXTAPIContext{
			Context: c,
			ext:     ext,
			Ext:     ext.Ext().V1(),
		})

		return api, nil
	})
}

// extReady checks that all required api services are available and established
func extReady(w *Context, apiServiceCache wapiregv1.APIServiceCache) bool {
	requiredAPIServices := []string{
		"v1.ext.cattle.io",
	}

	logrus.Debugf("[deferred-ext] %p checking EXT api-service availability and establishment status", w)

	for _, apiServiceName := range requiredAPIServices {
		apiService, err := apiServiceCache.Get(apiServiceName)
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Debugf("[deferred-ext] %p api-service %q not found, continuing to wait",
					w, apiServiceName)
				return false
			}
			logrus.Debugf("[deferred-ext] %p api-service %q: error during check: %v",
				w, apiServiceName, err)
			return false
		}

		established := false
		for _, condition := range apiService.Status.Conditions {
			if condition.Type == "Available" && condition.Status == "True" {
				established = true
				break
			}
		}

		if !established {
			logrus.Debugf("[deferred-ext] %p api-service %q: exists, not yet established, continuing to wait",
				w, apiServiceName)
			return false
		}

		logrus.Debugf("[deferred-ext] %p api-service %q is available and established", w, apiServiceName)
	}

	return true
}
