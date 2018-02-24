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
	if ns == nil {
		return nil
	}
	logrus.Debugf("nss Updated: %+v", *ns)
	projectID, ok := ns.Labels[nslabels.ProjectIDFieldLabel]
	if ok {
		logrus.Debugf("nss Updated: projectID=%v", projectID)
		return nss.npmgr.programNetworkPolicy(projectID)
	}
	return nil
}
