package planner

import (
	"encoding/json"
	"fmt"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/systemtemplate"
	corev1 "k8s.io/api/core/v1"
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

	var taints []corev1.Taint
	taintsAnn := machine.Annotations[TaintsAnnotation]
	if taintsAnn != "" {
		if err := json.Unmarshal([]byte(taintsAnn), &taints); err != nil {
			return nil, nil
		}
	}
	return systemtemplate.ForCluster(mgmtCluster, tokens[0].Status.Token, taints)
}
