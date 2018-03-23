package networkpolicy

import (
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
	logrus.Debugf("nss Sync: %+v", *ns)

	// program project isolation network policy
	projectID, ok := ns.Labels[nslabels.ProjectIDFieldLabel]
	if ok {
		logrus.Debugf("nss Sync: projectID=%v", projectID)
		if err := nss.npmgr.programNetworkPolicy(projectID); err != nil {
			logrus.Errorf("nsSyncer: Sync: error programming network policy: %v", err)
		}
	}

	// handle if hostNetwork policy is needed
	return nss.npmgr.handleHostNetwork()
}
