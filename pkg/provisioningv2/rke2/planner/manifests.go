package planner

import (
	"encoding/base64"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

func (p *Planner) getControlPlaneManifests(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (result []plan.File, _ error) {
	// NOTE: The agent does not have a means to delete files.  If you add a manifest that
	// may not exist in the future then you should create an empty file to "delete" the file
	if !isControlPlane(machine) {
		return nil, nil
	}

	clusterAgent, err := p.getClusterAgent(controlPlane, runtime.GetRuntime(controlPlane.Spec.KubernetesVersion), machine)
	if err != nil {
		return nil, err
	}
	result = append(result, clusterAgent)

	addons := p.getAddons(controlPlane, runtime.GetRuntime(controlPlane.Spec.KubernetesVersion))
	result = append(result, addons)

	return result, nil
}

func isDefaultTrueEnabled(b *bool) bool {
	return b == nil || *b
}

func (p *Planner) getClusterAgent(controlPlane *rkev1.RKEControlPlane, runtime string, machine *capi.Machine) (plan.File, error) {
	data, err := p.loadClusterAgent(controlPlane, machine)
	if err != nil {
		return plan.File{}, err
	}

	return plan.File{
		Content: base64.StdEncoding.EncodeToString(data),
		Path:    fmt.Sprintf("/var/lib/rancher/%s/server/manifests/rancher/cluster-agent.yaml", runtime),
		Dynamic: true,
	}, nil
}

func (p *Planner) getAddons(controlPlane *rkev1.RKEControlPlane, runtime string) plan.File {
	return plan.File{
		Content: base64.StdEncoding.EncodeToString([]byte(controlPlane.Spec.AdditionalManifest)),
		Path:    fmt.Sprintf("/var/lib/rancher/%s/server/manifests/rancher/addons.yaml", runtime),
		Dynamic: true,
	}
}
