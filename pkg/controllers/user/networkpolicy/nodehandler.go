package networkpolicy

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	rmgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type nodeHandler struct {
	npmgr            *netpolMgr
	clusterNamespace string
}

func (nh *nodeHandler) Sync(key string, machine *rmgmtv3.Node) error {
	if key == fmt.Sprintf("%s/%s", nh.clusterNamespace, nodesyncer.AllNodeKey) {
		logrus.Debugf("nodeHandler: Sync: key=%v", key)
		return nh.npmgr.handleHostNetwork()
	}
	return nil
}
