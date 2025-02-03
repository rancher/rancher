package ext

import (
	"fmt"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

type settingController struct {
	services    wranglercorev1.ServiceController
	apiservices wranglerapiregistrationv1.APIServiceController

	sniProvider *rotatingSNIProvider

	valueMu sync.Mutex
	value   string

	stopChan chan struct{}
}

// func (c *SettingController) sync(_ context.Context, _ string, obj *v3.Setting) (runtime.Object, error) {
func (c *settingController) sync(_ string, obj *v3.Setting) (*v3.Setting, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Name != settings.ImperativeApiExtension.Name {
		return obj, nil
	}

	c.valueMu.Lock()
	defer c.valueMu.Unlock()

	if c.value == obj.Value {
		return obj, nil
	}

	switch obj.Value {
	case "true":
		// todo: create extension API server resources
		logrus.Info("creating imperative extension apiserver resources")

		c.stopChan = make(chan struct{})

		go func() {
			if err := c.sniProvider.Run(c.stopChan); err != nil {
				logrus.Errorf("sni provider failed: %s", err)
			}
		}()

		if err := CreateOrUpdateService(c.services); err != nil {
			return nil, fmt.Errorf("failed to create or update APIService: %w", err)
		}
	default:
		logrus.Info("cleaning up imperative extension apiserver resources")

		close(c.stopChan)

		if err := CleanupExtensionAPIServer(c.services, c.apiservices); err != nil {
			return nil, err
		}
	}

	c.value = obj.Value

	return nil, nil
}
