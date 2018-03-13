package networkpolicy

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type projectNetworkPolicySyncer struct {
	npmgr *netpolMgr
}

// Sync invokes the Policy Handler to take care of installing the native network policies
func (pnplc *projectNetworkPolicySyncer) Sync(key string, pnp *v3.ProjectNetworkPolicy) error {
	if pnp == nil {
		return nil
	}
	logrus.Debugf("pnplc Sync pnp=%+v", pnp)
	return pnplc.npmgr.programNetworkPolicy(pnp.Namespace)
}
