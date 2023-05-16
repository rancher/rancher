package planner

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
)

// generateClusterAgentManifest generates a cluster agent manifest
func (p *Planner) generateClusterAgentManifest(controlPlane *rkev1.RKEControlPlane, entry *planEntry) ([]byte, error) {
	if controlPlane.Spec.ManagementClusterName == "local" {
		return nil, nil
	}

	return []byte{}, nil
}
