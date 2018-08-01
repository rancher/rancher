package networkpolicy

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type nodeHandler struct {
	npmgr            *netpolMgr
	clusterLister    v3.ClusterLister
	clusterNamespace string
}

func (nh *nodeHandler) Sync(key string, machine *v3.Node) error {
	if key == fmt.Sprintf("%s/%s", nh.clusterNamespace, nodesyncer.AllNodeKey) {
		disabled, err := isNetworkPolicyDisabled(nh.clusterNamespace, nh.clusterLister)
		if err != nil {
			return err
		}
		if disabled {
			return nil
		}
		logrus.Debugf("nodeHandler: Sync: key=%v", key)
		return nh.npmgr.handleHostNetwork(nh.clusterNamespace)
	}
	return nil
}
