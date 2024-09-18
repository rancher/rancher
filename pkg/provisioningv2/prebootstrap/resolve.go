package prebootstrap

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

func NewRetriever(clients *wrangler.Context) *Retriever {
	return &Retriever{mgmtClusters: clients.Mgmt.Cluster().Cache()}
}

type Retriever struct {
	mgmtClusters mgmtcontrollers.ClusterCache
}

func (r *Retriever) PreBootstrapClusters(cp *rkev1.RKEControlPlane) (bool, error) {
	mgmtCluster, err := r.mgmtClusters.Get(cp.Spec.ManagementClusterName)
	if err != nil {
		logrus.Warnf("[pre-bootstrap] failed to get management cluster [%v] for rke control plane [%v]: %v", cp.Spec.ManagementClusterName, cp.Name, err)
		return false, fmt.Errorf("failed to get mgmt Cluster %v: %w", cp.Spec.ManagementClusterName, err)
	}

	return !v3.ClusterConditionPreBootstrapped.IsTrue(mgmtCluster), nil
}
