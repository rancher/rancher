package networkpolicy

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

type nodeHandler struct {
	npmgr            *netpolMgr
	clusterLister    v3.ClusterLister
	clusterNamespace string
}

func (nh *nodeHandler) Sync(key string, machine *v3.Node) (runtime.Object, error) {
	if key == fmt.Sprintf("%s/%s", nh.clusterNamespace, nodesyncer.AllNodeKey) {
		disabled, err := isNetworkPolicyDisabled(nh.clusterNamespace, nh.clusterLister)
		if err != nil {
			return nil, err
		}
		if disabled {
			return nil, nil
		}
		logrus.Debugf("nodeHandler: Sync: key=%v", key)
		return nil, nh.npmgr.handleHostNetwork(nh.clusterNamespace)
	}
	return nil, nil
}
