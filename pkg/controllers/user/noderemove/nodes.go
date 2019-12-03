package noderemove

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	nodehelper "github.com/rancher/rancher/pkg/node"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

type nodeRemove struct {
	userNodes v1.NodeInterface
}

func Register(ctx context.Context, userContext *config.UserContext) {
	nsh := &nodeRemove{
		userNodes: userContext.Core.Nodes(""),
	}
	userContext.Management.Management.Nodes(userContext.ClusterName).AddClusterScopedLifecycle(ctx, "user-node-remove", userContext.ClusterName, nsh)
}

func (n *nodeRemove) Create(obj *v3.Node) (runtime.Object, error) {
	return obj, nil
}

func (n *nodeRemove) Remove(obj *v3.Node) (runtime.Object, error) {
	if nodehelper.IgnoreNode(obj.Status.NodeName, obj.Status.NodeLabels) {
		logrus.Debugf("Skipping v1.node removal for [%v] node", obj.Status.NodeName)
		return obj, nil
	}

	if obj.Status.NodeName != "" {
		n.userNodes.Delete(obj.Status.NodeName, nil)
	}
	return obj, nil
}

func (n *nodeRemove) Updated(obj *v3.Node) (runtime.Object, error) {
	return obj, nil
}
