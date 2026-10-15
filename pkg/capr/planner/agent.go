package planner

import (
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

// generateClusterAgentManifest generates the cluster agent manifest.
func (p *Planner) generateClusterAgentManifest(controlPlane *rkev1.RKEControlPlane, entry *planEntry) ([]byte, error) {
	if controlPlane.Spec.ManagementClusterName == "local" {
		return nil, nil
	}

	mgmtCluster, err := p.managementClusters.Get(controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	// todo(jhyde): mark the cluster as registered the first time the tunnel connects.
	// Use the default-token when initially rendering the manifest, then switch over to the system token once it exists.
	desiredTokenName := "default-token"
	if apimgmtv3.ClusterConditionAgentDeployed.IsTrue(mgmtCluster) {
		desiredTokenName = "system"
	}

	tokens, err := p.clusterRegistrationTokenCache.GetByIndex(ClusterRegToken, mgmtCluster.Name)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no cluster registration token found")
	}

	var crt *apimgmtv3.ClusterRegistrationToken
	for _, t := range tokens {
		if t.Name == desiredTokenName {
			crt = t
			break
		}
	}

	if crt == nil {
		return nil, fmt.Errorf("could not find cluster registration token %s", desiredTokenName)
	}

	taints, err := getTaints(entry, controlPlane)
	if err != nil {
		return nil, err
	}

	token, err := clusterregistrationtoken.GetTokenFromSecret(p.secretCache, crt)
	if err != nil {
		return nil, err
	}

	return systemtemplate.ForCluster(mgmtCluster, token, taints, p.secretCache)
}
