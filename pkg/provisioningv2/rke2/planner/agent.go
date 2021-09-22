package planner

import (
	"fmt"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/systemtemplate"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

func (p *Planner) loadClusterAgent(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) ([]byte, error) {
	if controlPlane.Spec.ManagementClusterName == "local" {
		return nil, nil
	}

	tokens, err := p.clusterRegistrationTokenCache.GetByIndex(clusterRegToken, controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no cluster registration token found")
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].Name < tokens[j].Name
	})

	mgmtCluster, err := p.managementClusters.Get(controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	taints, err := getTaints(machine, runtime.GetRuntime(controlPlane.Spec.KubernetesVersion))
	if err != nil {
		return nil, err
	}

	return systemtemplate.ForCluster(mgmtCluster, tokens[0].Status.Token, taints)
}
