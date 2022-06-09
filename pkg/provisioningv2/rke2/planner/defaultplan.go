package planner

import (
	"encoding/base64"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
)

// commonNodePlan returns a "default" node plan with the corresponding registry configuration.
// It will append to the node plan passed in through options.
func (p *Planner) commonNodePlan(controlPlane *rkev1.RKEControlPlane, options plan.NodePlan) (plan.NodePlan, error) {
	if controlPlane.Spec.Registries == nil {
		return options, nil
	}

	registryConfig, files, err := p.toRegistryConfig(rke2.GetRuntime(controlPlane.Spec.KubernetesVersion),
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
