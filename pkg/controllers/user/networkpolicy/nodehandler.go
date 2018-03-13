package networkpolicy

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type nodeHandler struct {
	npmgr *netpolMgr
}

func (nh *nodeHandler) Sync(key string, node *corev1.Node) error {
	if node == nil {
		return nil
	}
	logrus.Debugf("nodeHandler: Sync: %+v", *node)
	return nh.npmgr.handleHostNetwork()
}
