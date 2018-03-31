package networkpolicy

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type projectNetworkPolicySyncer struct {
	npmgr *netpolMgr
}

// Sync invokes the Policy Handler to take care of installing the native network policies
func (pnps *projectNetworkPolicySyncer) Sync(key string, pnp *v3.ProjectNetworkPolicy) error {
	if pnp == nil || pnp.DeletionTimestamp != nil {
		return nil
	}
	logrus.Debugf("projectNetworkPolicySyncer: Sync: pnp=%+v", pnp)
	return pnps.npmgr.programNetworkPolicy(pnp.Namespace)
}
