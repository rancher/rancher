package planner

import (
	"encoding/base64"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
)

func commonNodePlan(secrets corecontrollers.SecretCache, controlPlane *rkev1.RKEControlPlane, options plan.NodePlan) (plan.NodePlan, error) {
	if controlPlane.Spec.Registries == nil {
		return options, nil
	}

	registryConfig, files, err := toRegistryConfig(secrets, runtime.GetRuntime(controlPlane.Spec.KubernetesVersion),
		controlPlane.Namespace, controlPlane.Spec.Registries)
	if err != nil {
		return plan.NodePlan{}, err
	}

	options.Files = append(append([]plan.File{{
		Content: base64.StdEncoding.EncodeToString(registryConfig),
		Path:    "/etc/rancher/agent/registries.yaml",
		Dynamic: true,
	}}, files...), options.Files...)

	return options, nil
}
