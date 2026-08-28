package planner

import (
	"fmt"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

// generateClusterAgentManifest generates a cluster agent manifest
func (p *Planner) generateClusterAgentManifest(controlPlane *rkev1.RKEControlPlane, entry *planEntry) ([]byte, error) {
	if controlPlane.Spec.ManagementClusterName == "local" {
		return nil, nil
	}

	tokens, err := p.clusterRegistrationTokenCache.GetByIndex(ClusterRegToken, controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no cluster registration token found")
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].Name < tokens[j].Name
	})

	// Prefer the cluster's default token, falling back to the first by name. The rendered
	// token determines the name of the cattle-credentials-<hash> secret,
	// CATTLE_CREDENTIAL_NAME and the agent pod's credential volume, so the clusterdeploy
	// controller has to render the same token or its apply of this same Deployment mutates
	// it and rolls the agent. Sorting by name alone is not enough: a user generating a
	// registration command creates a crt-* token that sorts ahead of default-token, which
	// would silently change the credential secret's name and roll the agent.
	regToken := tokens[0]
	for _, t := range tokens {
		if t.Name == capr.DefaultClusterRegistrationTokenName {
			regToken = t
			break
		}
	}

	mgmtCluster, err := p.managementClusters.Get(controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	taints, err := getTaints(entry, controlPlane)
	if err != nil {
		return nil, err
	}

	token, err := crt.GetTokenFromSecret(p.secretCache, regToken)
	if err != nil {
		return nil, err
	}

	return systemtemplate.ForCluster(mgmtCluster, token, taints, p.secretCache)
}
