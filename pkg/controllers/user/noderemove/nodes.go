package noderemove

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

type nodeRemove struct {
	userNodes v1.NodeInterface
}

func Register(userContext *config.UserContext) {
	nsh := &nodeRemove{
		userNodes: userContext.Core.Nodes(""),
	}
	userContext.Management.Management.Nodes(userContext.ClusterName).AddLifecycle("user-node-remove", nsh)
}

func (n *nodeRemove) Create(obj *v3.Node) (*v3.Node, error) {
	return obj, nil
}

func (n *nodeRemove) Remove(obj *v3.Node) (*v3.Node, error) {
	if obj.Status.NodeName != "" {
		err := n.userNodes.Delete(obj.Status.NodeName, nil)
		return obj, err
	}
	return obj, nil
}

func (n *nodeRemove) Updated(obj *v3.Node) (*v3.Node, error) {
	return obj, nil
}
