package prebootstrap

import (
	"encoding/base64"
	"fmt"
	"path"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

func NewRetriever(clients *wrangler.Context) *Retriever {
	return &Retriever{
		mgmtClusterCache:              clients.Mgmt.Cluster().Cache(),
		clusterRegistrationTokenCache: clients.Mgmt.ClusterRegistrationToken().Cache(),
		secretCache:                   clients.Core.Secret().Cache(),
	}

}

type Retriever struct {
	mgmtClusterCache              mgmtcontrollers.ClusterCache
	clusterRegistrationTokenCache mgmtcontrollers.ClusterRegistrationTokenCache
	secretCache                   corecontrollers.SecretCache
}

func (r *Retriever) GeneratePreBootstrapClusterAgentManifest(controlPlane *rkev1.RKEControlPlane) ([]plan.File, error) {
	shouldDo, err := r.preBootstrapCluster(controlPlane)
	if !shouldDo {
		return nil, nil
	}

	tokens, err := r.clusterRegistrationTokenCache.GetByIndex(planner.ClusterRegToken, controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no cluster registration token found")
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].Name < tokens[j].Name
	})

	mgmtCluster, err := r.mgmtClusterCache.Get(controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get mgmt Cluster %v: %w", controlPlane.Spec.ManagementClusterName, err)
	}

	// passing in nil for taints since prebootstrapping involves specific taints to uninitialized nodes
	data, err := systemtemplate.ForCluster(mgmtCluster, tokens[0].Status.Token, nil, r.secretCache)
	if err != nil {
		return nil, fmt.Errorf("failed to generate pre-bootstrap cluster-agent manifest: %w", err)
	}

	return []plan.File{{
		Content: base64.StdEncoding.EncodeToString(data),
		Path:    path.Join(capr.GetDistroDataDir(controlPlane), "server/manifests/rancher/cluster-agent.yaml"),
		Dynamic: true,
		Minor:   true,
	}}, nil
}

func (r *Retriever) preBootstrapCluster(cp *rkev1.RKEControlPlane) (bool, error) {
	mgmtCluster, err := r.mgmtClusterCache.Get(cp.Spec.ManagementClusterName)
	if err != nil {
		logrus.Warnf("[pre-bootstrap] failed to get management cluster [%v] for rke control plane [%v]: %v", cp.Spec.ManagementClusterName, cp.Name, err)
		return false, fmt.Errorf("failed to get mgmt Cluster %v: %w", cp.Spec.ManagementClusterName, err)
	}

	return capr.PreBootstrap(mgmtCluster), nil
}
