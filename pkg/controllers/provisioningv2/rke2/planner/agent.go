package planner

import (
	"fmt"
	"sort"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/systemtemplate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateClusterAgentManifest generates a cluster agent manifest
func (p *Planner) generateClusterAgentManifest(controlPlane *rkev1.RKEControlPlane, entry *planEntry) ([]byte, error) {
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

	taints, err := getTaints(entry)
	if err != nil {
		return nil, err
	}

	var formattedPrivateRegistry map[string][]byte
	privateRegistrySecret := mgmtCluster.GetSecret(v3.ClusterPrivateRegistrySecret)
	privateRegistryURL := mgmtCluster.GetSecret(v3.ClusterPrivateRegistryURL)
	if privateRegistrySecret != "" && privateRegistryURL != "" {
		// cluster level registry has been defined and should be used when generating agent manifest.
		// This ensures images pulled by the agent (e.g. shell) come from the correct registry.
		privateRegistries, err := p.secretClient.Get(fleet.ClustersDefaultNamespace, privateRegistrySecret, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		// transform the registry secrets for ProvV2 as they only have 'username' and 'password' fields, but SystemTemplate expects the credentialprovider.DockerConfigJSON type
		formattedPrivateRegistry, err = util.TransformProvV2RegistryCredentialsToDockerConfigJSON(privateRegistryURL, privateRegistries)
		if err != nil {
			return nil, err
		}
	}

	return systemtemplate.ForCluster(mgmtCluster, tokens[0].Status.Token, taints, formattedPrivateRegistry)
}
