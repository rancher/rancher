package networkpolicy

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type nsSyncer struct {
	npmgr *netpolMgr
}

// Sync invokes Policy Handler to program the native network policies
func (nss *nsSyncer) Sync(key string, ns *corev1.Namespace) error {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil
	}
	logrus.Debugf("nsSyncer: Sync: %v, %+v", ns.Name, *ns)

	// program project isolation network policy
	projectID, ok := ns.Labels[nslabels.ProjectIDFieldLabel]
	if ok {
		logrus.Debugf("nsSyncer: Sync: ns=%v projectID=%v", ns.Name, projectID)
		if err := nss.npmgr.programProjectNetworkPolicy(projectID); err != nil {
			return fmt.Errorf("nsSyncer: Sync: error programming network policy: %v (ns=%v, projectID=%v), ", err, ns.Name, projectID)
		}
	}

	// handle if hostNetwork policy is needed
	return nss.npmgr.handleHostNetwork()
}
