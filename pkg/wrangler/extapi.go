package wrangler

import (
	"context"

	extapi "github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManageDeferredEXTAPIContext handles the deferrals requiring the EXT api-service.
func (w *Context) ManageDeferredEXTAPIContext(ctx context.Context) {
	w.ManageDeferrals(ctx,
		"EXT api-service availability",
		w.DeferredEXTAPIRegistration,
		func(w *Context) bool {
			requiredAPIServices := []string{
				"v1.ext.cattle.io",
			}

			logrus.Debugf("[deferred-extapi] %p checking EXT api-service availability and establishment status", w)

			for _, apiServiceName := range requiredAPIServices {
				apiService, err := w.API.APIService().Get(apiServiceName, metav1.GetOptions{})
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
		},
		func(w *Context) {
			logrus.Debugf("[deferred-extapi] %p ext factory create", w)

			ext, err := extapi.NewFactoryFromConfigWithOptions(w.RESTConfig, &generic.FactoryOptions{
				SharedControllerFactory: w.ControllerFactory,
			})
			if err != nil {
				logrus.Fatalf("Encountered unexpected panic while creating ext factory: %v", err)
			}

			w.ext = ext
			w.Ext = ext.Ext().V1()
		})
}
