package planner

import (
	"fmt"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	"github.com/rancher/rancher/pkg/systemtemplate"
	rketypes "github.com/rancher/rke/types"
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

	// assemble private registry
	var registry *rketypes.PrivateRegistry
	privateRegistrySecretRef := mgmtCluster.GetSecret("PrivateRegistrySecret")
	privateRegistryURL := mgmtCluster.Spec.ClusterSecrets.PrivateRegistryURL
	ecrSecretRef := mgmtCluster.Spec.ClusterSecrets.PrivateRegistryECRSecret
	if privateRegistrySecretRef != "" || privateRegistryURL != "" || ecrSecretRef != "" {
		// Assemble private registry secrets here instead of directly on the cluster object
		spec := *mgmtCluster.Spec.DeepCopy()
		spec, err = secretmigrator.AssemblePrivateRegistryCredential(privateRegistrySecretRef, privateRegistryURL, secretmigrator.ClusterType, mgmtCluster.Name, spec, p.secretCache)
		if err != nil {
			return nil, err
		}
		spec, err = secretmigrator.AssemblePrivateRegistryECRCredential(ecrSecretRef, secretmigrator.ClusterType, mgmtCluster.Name, mgmtCluster.Spec, p.secretCache)
		if err != nil {
			return nil, err
		}
		registry = util.GetPrivateRegistry(spec.RancherKubernetesEngineConfig)
	}

	return systemtemplate.ForCluster(mgmtCluster, tokens[0].Status.Token, registry, taints)
}
