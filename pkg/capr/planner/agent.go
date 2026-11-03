package planner

import (
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

// generateClusterAgentManifest generates the cluster agent manifest.
// The generated manifest is dropped into the RKE2/K3s server/manifests directory, which are reconciled on startup.
// As these manifests are reconciled each time the service is restarted, it is imperative this manifest mirrors the
// manifest deployed by [github.com/rancher/rancher/pkg/controllers/management/clusterdeploy] in order to prevent
// thrashing.
func (p *Planner) generateClusterAgentManifest(controlPlane *rkev1.RKEControlPlane, entry *planEntry) ([]byte, error) {
	if controlPlane.Spec.ManagementClusterName == "local" {
		return nil, nil
	}

	mgmtCluster, err := p.managementClusters.Get(controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	tokens, err := p.clusterRegistrationTokenCache.GetByIndex(ClusterRegToken, mgmtCluster.Name)
	if err != nil {
		return nil, err
	}

	crt, err := ClusterAgentRegistrationToken(mgmtCluster, tokens)
	if err != nil {
		return nil, err
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

const (
	// defaultRegistrationTokenName is the registration token created with the cluster, which its agent connects
	// with initially.
	defaultRegistrationTokenName = "default-token"
	// systemRegistrationTokenName is the registration token clusterDeploy deploys the agent with.
	systemRegistrationTokenName = "system"
)

// ClusterAgentRegistrationToken returns the registration token, among tokens, that the agent of mgmtCluster is to be
// rendered with. That is the default-token until an agent has connected, and the system token from then on, the same
// as the agent deployed by [github.com/rancher/rancher/pkg/controllers/management/clusterdeploy]: it deploys the
// agent through that connection, always with the system token. Rendering the default-token past that point would
// make the two manifests differ, and each would roll out its own agent.
//
// Once an agent has connected and until the system token exists, which clusterDeploy creates right before deploying
// the agent, it returns an errWaiting rather than falling back to the default-token. clusterDeploy marks the agent
// deployed right after, which updates the management cluster and so plans the control plane again.
func ClusterAgentRegistrationToken(mgmtCluster *apimgmtv3.Cluster, tokens []*apimgmtv3.ClusterRegistrationToken) (*apimgmtv3.ClusterRegistrationToken, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no cluster registration token found")
	}

	desiredTokenName := defaultRegistrationTokenName
	if agentHasConnected(mgmtCluster) {
		desiredTokenName = systemRegistrationTokenName
	}

	for _, t := range tokens {
		if t.Name == desiredTokenName {
			return t, nil
		}
	}

	if desiredTokenName == systemRegistrationTokenName {
		return nil, errWaitingf("waiting for cluster registration token %s of cluster %s", desiredTokenName, mgmtCluster.Name)
	}
	return nil, fmt.Errorf("could not find cluster registration token %s", desiredTokenName)
}

// agentHasConnected reports whether an agent of cluster has connected to Rancher, at which point clusterDeploy can
// deploy the agent with the system token. Connected alone doesn't say: it is held False until the cluster is
// pre-bootstrapped, and goes False again while the agent is replaced. Pre-bootstrapping is done through a connected
// agent, so PreBootstrapped says so first, and AgentDeployed keeps saying so once clusterDeploy has deployed it.
func agentHasConnected(cluster *apimgmtv3.Cluster) bool {
	return apimgmtv3.ClusterConditionConnected.IsTrue(cluster) ||
		apimgmtv3.ClusterConditionPreBootstrapped.IsTrue(cluster) ||
		apimgmtv3.ClusterConditionAgentDeployed.IsTrue(cluster)
}
