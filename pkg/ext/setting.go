package ext

import (
	"fmt"
	"strings"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ext/listener"
	"github.com/rancher/rancher/pkg/settings"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

type settingController struct {
	services    wranglercorev1.ServiceController
	apiservices wranglerapiregistrationv1.APIServiceController

	sniProvider *rotatingSNIProvider

	authenticator *ToggleUnionAuthenticator

	listener *listener.Listener

	stopChanMu sync.Mutex
	stopChan   chan struct{}
}

func (c *settingController) sync(_ string, obj *v3.Setting) (*v3.Setting, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Name != settings.ImperativeApiExtension.Name {
		return obj, nil
	}

	c.stopChanMu.Lock()
	defer c.stopChanMu.Unlock()

	switch strings.ToLower(obj.Value) {
	case "true":
		logrus.Info("enabling imperative apiserver")

		if c.stopChan != nil {
			logrus.Debug("imperative extension apiserver already enabled")
			break
		}
		c.stopChan = make(chan struct{})

		c.authenticator.SetEnabled(authenticatorNameSteveDefault, true)

		go func() {
			if err := c.sniProvider.Run(c.stopChan); err != nil {
				logrus.Errorf("sni provider failed: %s", err)
			}
		}()

		if err := c.listener.Start(); err != nil {
			return nil, fmt.Errorf("failed to start listener: %w", err)
		}

		if err := CreateOrUpdateService(c.services); err != nil {
			return nil, fmt.Errorf("failed to create or update APIService: %w", err)
		}
	default:
		logrus.Info("disabling up imperative apiserver")

		if c.stopChan == nil {
			logrus.Debug("imperative extension apiserver is not enabled")
			break
		}
		close(c.stopChan)
		c.stopChan = nil

		if err := c.listener.Stop(); err != nil {
			return nil, fmt.Errorf("failed to stop listener: %w", err)
		}

		c.authenticator.SetEnabled(authenticatorNameSteveDefault, false)

		if err := CleanupExtensionAPIServer(c.services, c.apiservices); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
