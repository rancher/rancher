package controllers

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mgmgv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/telemetry/controllers/secretrequest"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterControllers(ctx context.Context, wContext *wrangler.Context) error {
	// TODO(alex): idea on better way to do this
	// We could do k8s RBAC bindings here instead of this
	projects, err := wContext.Mgmt.Project().List("local", v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to register telemetry controllers: %w", err)
	}

	var systemProject *mgmgv3.Project
	for _, project := range projects.Items {
		if project.Spec.DisplayName != "System" {
			continue
		}
		systemProject = &project
	}
	if systemProject == nil {
		return fmt.Errorf("no system project found")
	}

	// TODO: I think because this is called before rancher starts we don't need to start controllers manually
	secretrequest.Register(
		ctx,
		wContext.Telemetry.SecretRequest(),
		wContext.Telemetry.SecretRequest().Cache(),
		systemProject,
		wContext.Core.Namespace(),
		wContext.Core.Secret(),
	)

	return nil
}
