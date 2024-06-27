package planner

import (
	"encoding/base64"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
)

// commonNodePlan returns a "default" node plan with the corresponding registry configuration.
// It will append to the node plan passed in through options.
func (p *Planner) commonNodePlan(controlPlane *rkev1.RKEControlPlane, np plan.NodePlan) (plan.NodePlan, registries, error) {
	if controlPlane.Spec.Registries == nil {
		return np, registries{}, nil
	}

	reg, err := p.renderRegistries(controlPlane)
	if err != nil {
		return plan.NodePlan{}, registries{}, err
	}

	// Render the registries.yaml file for the rancher-system-agent. The registries.yaml file for the respective distribution should be rendered elsewhere
	// (at config file rendering)
	np.Files = append(np.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(reg.registriesFileRaw),
		Path:    "/etc/rancher/agent/registries.yaml",
		Dynamic: true,
	})
	// Add the corresponding certificate files (if they exist)
	np.Files = append(np.Files, reg.certificateFiles...)

	return np, reg, nil
}
