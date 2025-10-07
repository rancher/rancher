package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	mgmgv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/telemetry/controllers/secretrequest"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

var SystemProjectBackoff = wait.Backoff{
	Steps:    10,
	Duration: 15 * time.Second,
	Factor:   2.0,
	Jitter:   0.1,
}

func RegisterControllers(ctx context.Context, wContext *wrangler.Context, telemetryManager telemetry.TelemetryExporterManager) error {
	// TODO(dan): we could do k8s RBAC bindings here instead of this
	var systemProject *mgmgv3.Project

	if initErr := retry.OnError(SystemProjectBackoff, func(err error) bool {
		logrus.Errorf("failed to register telemetry controller, will retry: %v", err)
		return true
	}, func() error {
		projects, err := wContext.Mgmt.Project().List("local", v1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list projects: %v", err)
		}

		for _, project := range projects.Items {
			if project.Spec.DisplayName != "System" {
				continue
			}
			systemProject = &project
		}

		if systemProject == nil {
			return fmt.Errorf("no system project found")
		}

		return nil
	}); initErr != nil {
		return initErr
	}

	secretrequest.Register(
		ctx,
		wContext.Telemetry.SecretRequest(),
		wContext.Telemetry.SecretRequest().Cache(),
		systemProject,
		wContext.Core.Namespace(),
		wContext.Core.Secret(),
		telemetryManager,
	)

	return nil
}
