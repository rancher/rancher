package networkpolicy

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type serviceHandler struct {
	npmgr *netpolMgr
}

func (sh *serviceHandler) Sync(key string, service *corev1.Service) error {
	if service == nil || service.DeletionTimestamp != nil {
		return nil
	}
	logrus.Debugf("serviceHandler: Sync: %+v", *service)
	return sh.npmgr.nodePortsUpdateHandler(service)
}
