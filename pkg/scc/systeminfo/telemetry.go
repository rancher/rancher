package systeminfo

import (
	"errors"
	"iter"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3ctrl "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

type ClusterID string
type NodeID string
type NodeRole string

const (
	NodeRoleEtcd    NodeRole = "etcd"
	NodeRoleWorker  NodeRole = "worker"
	NodeRoleControl NodeRole = "control-plane"
	NodeRoleUnknown NodeRole = "unknown"
)

var (
	ErrNoCPUReported = errors.New("no CPU reported for cluster")
	ErrNoMemReported = errors.New("no mem reported for cluster")
)

func emptyIter[T any]() iter.Seq[T] {
	return func(_ func(T) bool) {}
}

func emptyIter2[K, V any]() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {}
}

type TelemetryGatherer struct {
	namespace    v1core.NamespaceCache
	nodeCache    v3ctrl.NodeCache
	clusterCache v3ctrl.ClusterCache
}

type ClusterTelemetry interface {
	CpuCores() (int, error)
	MemoryCapacity() (resource.Quantity, error)
	PerNodeTelemetry() iter.Seq2[NodeID, NodeTelemetry]
}

type NodeTelemetry interface {
	Role() NodeRole
	CpuCores() (int, error)
	MemoryCapacity() (resource.Quantity, error)
}

type nodeTelemetryImpl struct {
	n *v3.Node
}

var _ NodeTelemetry = (*nodeTelemetryImpl)(nil)

func (n *nodeTelemetryImpl) Role() NodeRole {
	if n.n.Spec.Etcd {
		return NodeRoleEtcd
	}
	if n.n.Spec.ControlPlane {
		return NodeRoleControl
	}
	if n.n.Spec.Worker {
		return NodeRoleWorker
	}

	return NodeRoleUnknown
}

func (n *nodeTelemetryImpl) CpuCores() (int, error) {
	cpuQ := n.n.Status.InternalNodeStatus.Capacity.Cpu()
	if cpuQ == nil {
		return 0, ErrNoCPUReported
	}
	return cpuQ.Size(), nil
}

func (n *nodeTelemetryImpl) MemoryCapacity() (resource.Quantity, error) {
	memQ := n.n.Status.InternalNodeStatus.Capacity.Memory()
	if memQ == nil {
		return resource.Quantity{}, ErrNoMemReported
	}
	return *memQ, nil
}

type RancherManagerTelemetry interface {
	ClusterCount() int
	PerClusterTelemetry() iter.Seq2[ClusterID, ClusterTelemetry]
}

type rancherTelemetryImpl struct {
	clusterList []*v3.Cluster
	nodeList    map[string][]*v3.Node
}

func (r *rancherTelemetryImpl) ClusterCount() int {
	return len(r.clusterList)
}

type clusterTelemetryImpl struct {
	*v3.Cluster
	associatedNodes []*v3.Node
}

var _ ClusterTelemetry = (*clusterTelemetryImpl)(nil)

func (c *clusterTelemetryImpl) CpuCores() (int, error) {
	cpuQ := c.Status.Capacity.Cpu()
	if cpuQ == nil {
		return 0, ErrNoCPUReported
	}

	return cpuQ.Size(), nil
}

func (c *clusterTelemetryImpl) MemoryCapacity() (resource.Quantity, error) {
	memQ := c.Status.Capacity.Memory()
	if memQ == nil {
		return resource.Quantity{}, ErrNoMemReported
	}
	return *memQ, nil
}

func (c *clusterTelemetryImpl) PerNodeTelemetry() iter.Seq2[NodeID, NodeTelemetry] {
	return func(yield func(NodeID, NodeTelemetry) bool) {
		for _, n := range c.associatedNodes {
			n.ObjClusterName()
			if !yield(NodeID(n.Name), &nodeTelemetryImpl{
				n: n,
			}) {
				break
			}
		}
	}
}

func (r *rancherTelemetryImpl) PerClusterTelemetry() iter.Seq2[ClusterID, ClusterTelemetry] {
	return func(yield func(ClusterID, ClusterTelemetry) bool) {
		for _, cl := range r.clusterList {
			nodes, ok := r.nodeList[cl.Name]
			if !ok {
				logrus.Warnf("detected no associated nodes for cluster : %s", cl.Name)
			}

			if !yield(ClusterID(cl.Name), &clusterTelemetryImpl{
				Cluster:         cl,
				associatedNodes: nodes,
			}) {
				break
			}
		}
	}
}

var _ RancherManagerTelemetry = (*rancherTelemetryImpl)(nil)

func (t *TelemetryGatherer) GetClusterTelemetry() (RancherManagerTelemetry, error) {
	// TODO : don't need this, we know the namespaces to look up from cluster names
	nsList, err := t.namespace.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	cls, err := t.clusterCache.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	nodeMap := map[string][]*v3.Node{}

	for _, ns := range nsList {
		nodePerNs, err := t.nodeCache.List(ns.Name, labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, node := range nodePerNs {
			clName := node.ObjClusterName()
			if _, ok := nodeMap[clName]; !ok {
				nodeMap[clName] = []*v3.Node{}
			}
			nodeMap[clName] = append(nodeMap[clName], node)
		}
	}
	return &rancherTelemetryImpl{
		clusterList: cls,
		nodeList:    nodeMap,
	}, nil
}
