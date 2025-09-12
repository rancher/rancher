package wrangler

import (
	"context"
	"sync/atomic"

	extapi "github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io"
	wapiregv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

// manageDeferredEXTAPIContext spawns a waiter for the availability of the EXT api-service. On success it
// executes all the deferred functions, after seting up the necessary fields of the wrangler structure.
func (w *Context) manageDeferredEXTAPIContext(ctx context.Context) {

	logrus.Info("[deferred-extapi] manageDeferredEXTAPIContext starting waiter for EXT api-service availability")
	var done atomic.Bool
	w.API.APIService().OnChange(ctx, "extapi-deferred-registration", func(key string, api *apiregv1.APIService) (*apiregv1.APIService, error) {
		if done.Load() {
			return api, nil
		}

		if !extReady(w, w.API.APIService().Cache()) {
			return api, nil
		}

		if !done.CompareAndSwap(false, true) {
			return api, nil
		}

		logrus.Debugf("[deferred-extapi] %p ext factory create", w)

		ext, err := extapi.NewFactoryFromConfigWithOptions(w.RESTConfig, &generic.FactoryOptions{
			SharedControllerFactory: w.ControllerFactory,
		})
		if err != nil {
			logrus.Fatalf("Encountered unexpected error while creating ext factory: %v", err)
		}

		func() {
			logrus.Debugf("[deferred-extapi] %p complete ext fields of context", w)

			w.extMutex.Lock()
			defer w.extMutex.Unlock()

			w.ext = ext
			w.Ext = ext.Ext().V1()
		}()

		go func() {
			logrus.Debugf("[deferred-extapi] %p begin execution of defered functions", w)

			err = w.DeferredEXTAPIRegistration.Run(ctx)
			if err != nil {
				logrus.Fatalf("failed to run loop during deferred registration: %v", err)
			}

			logrus.Debugf("[deferred-extapi] %p ended execution of defered functions", w)
		}()

		return api, nil
	})
}

// extReady checks that all required api services are available and established
func extReady(w *Context, apiServiceCache wapiregv1.APIServiceCache) bool {
	requiredAPIServices := []string{
		"v1.ext.cattle.io",
	}

	logrus.Debugf("[deferred-extapi] %p checking EXT api-service availability and establishment status", w)

	for _, apiServiceName := range requiredAPIServices {
		apiService, err := apiServiceCache.Get(apiServiceName)
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Debugf("[deferred-extapi] %p api-service %q not found, continuing to wait",
					w, apiServiceName)
				return false
			}
			logrus.Debugf("[deferred-extapi] %p api-service %q: error during check: %v",
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
			logrus.Debugf("[deferred-extapi] %p api-service %q: exists, not yet established, continuing to wait",
				w, apiServiceName)
			return false
		}

		logrus.Debugf("[deferred-extapi] %p api-service %q is available and established", w, apiServiceName)
	}

	return true
}
