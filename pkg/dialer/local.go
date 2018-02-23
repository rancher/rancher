package dialer

import (
	"net"

	"fmt"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	node2 "k8s.io/kubernetes/pkg/util/node"
)

const (
	nodeAccessIndexer = "nodeAccess"
)

var (
	preferredNodeAddress = []v1.NodeAddressType{
		v1.NodeInternalIP,
		v1.NodeExternalIP,
		v1.NodeHostName,
	}
)

func nodeIndexer(obj interface{}) ([]string, error) {
	if node, ok := obj.(*v3.Node); ok {
		addr, err := node2.GetPreferredNodeAddress(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: node.Status.NodeLabels,
			},
			Status: node.Status.InternalNodeStatus,
		}, preferredNodeAddress)
		return []string{addr}, err
	}

	return nil, nil
}

func (f *Factory) LocalClusterDialer() dialer.Dialer {
	return f.dialLocal
}

func (f *Factory) dialLocal(network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	objs, err := f.localNodeController.Informer().GetIndexer().ByIndex(nodeAccessIndexer, host)
	if err != nil {
		return nil, err
	}

	for _, obj := range objs {
		if node, ok := obj.(*v3.Node); ok {
			return f.TunnelServer.Dial(node.Name, 15*time.Second, network, address)
		}
	}

	return nil, fmt.Errorf("failed to find tunnel for: %v", address)
}
