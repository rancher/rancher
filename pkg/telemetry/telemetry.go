package telemetry

import (
	"errors"
	"iter"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3ctrl "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
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

const (
	localClusterID = "local"
)

var (
	ErrNoCPUReported  = errors.New("no CPU reported for cluster")
	ErrCpuCoresFormat = errors.New("encountered unexpected format for CPU cores")
	ErrNoMemReported  = errors.New("no mem reported for cluster")
	ErrMemBytesFormat = errors.New("encountered unexpected format for memory bytes")
)

func emptyIter[T any]() iter.Seq[T] {
	return func(_ func(T) bool) {}
}

func emptyIter2[K, V any]() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {}
}

type RancherManagerTelemetry interface {
	// Management

	ManagedClusterCount() int
	PerManagedClusterTelemetry() iter.Seq2[ClusterID, ClusterTelemetry]

	// Local cluster-specific

	RancherVersion() string
	LocalNodeCount() int
	LocalClusterTelemetry() ClusterTelemetry
}

type ClusterTelemetry interface {
	ComputeTelmetry
	PerNodeTelemetry() iter.Seq2[NodeID, NodeTelemetry]
}

type NodeTelemetry interface {
	Role() NodeRole
	ComputeTelmetry
	SystemTelemetry
}

type SystemTelemetry interface {
	OS() string
	ContainerRuntime() string
	KernelVersion() string
	CpuArchitecture() string
}

type ComputeTelmetry interface {
	CpuCores() (int, error)
	MemoryCapacityBytes() (int, error)
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
	return handleCpuCores(*cpuQ)
}

func (n *nodeTelemetryImpl) MemoryCapacityBytes() (int, error) {
	memQ := n.n.Status.InternalNodeStatus.Capacity.Memory()
	if memQ == nil {
		return 0, ErrNoMemReported
	}
	return handleMemBytes(*memQ)
}

func (n *nodeTelemetryImpl) nodeInfo() v1.NodeSystemInfo {
	return n.n.Status.InternalNodeStatus.NodeInfo
}

func (n *nodeTelemetryImpl) OS() string {
	return n.nodeInfo().OperatingSystem
}

func (n *nodeTelemetryImpl) CpuArchitecture() string {
	return n.nodeInfo().Architecture
}

func (n *nodeTelemetryImpl) ContainerRuntime() string {
	return n.nodeInfo().ContainerRuntimeVersion
}

func (n *nodeTelemetryImpl) KernelVersion() string {
	return n.nodeInfo().KernelVersion
}

type rancherTelemetryImpl struct {
	rancherVersion string

	localCluster    *v3.Cluster
	localNodes      []*v3.Node
	managedClusters []*v3.Cluster
	managedNodes    map[ClusterID][]*v3.Node
}

func (r *rancherTelemetryImpl) ManagedClusterCount() int {
	return 1 + len(r.managedClusters)
}

func (r *rancherTelemetryImpl) LocalNodeCount() int {
	return len(r.localNodes)
}

func (r *rancherTelemetryImpl) LocalClusterTelemetry() ClusterTelemetry {
	return &clusterTelemetryImpl{
		Cluster:         r.localCluster,
		associatedNodes: r.localNodes,
	}
}

func (r *rancherTelemetryImpl) PerManagedClusterTelemetry() iter.Seq2[ClusterID, ClusterTelemetry] {
	return func(yield func(ClusterID, ClusterTelemetry) bool) {
		for _, cl := range r.managedClusters {
			nodes, ok := r.managedNodes[ClusterID(cl.Name)]
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

func (r *rancherTelemetryImpl) RancherVersion() string {
	return r.rancherVersion
}

var _ RancherManagerTelemetry = (*rancherTelemetryImpl)(nil)

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
	return handleCpuCores(*cpuQ)
}

func (c *clusterTelemetryImpl) MemoryCapacityBytes() (int, error) {
	memQ := c.Status.Capacity.Memory()
	if memQ == nil {
		return 0, ErrNoMemReported
	}
	return handleMemBytes(*memQ)
}

func (c *clusterTelemetryImpl) PerNodeTelemetry() iter.Seq2[NodeID, NodeTelemetry] {
	return func(yield func(NodeID, NodeTelemetry) bool) {
		for _, n := range c.associatedNodes {
			if !yield(NodeID(n.Name), &nodeTelemetryImpl{
				n: n,
			}) {
				break
			}
		}
	}
}

type TelemetryGatherer struct {
	rancherVersion string
	nodeCache      v3ctrl.NodeCache
	clusterCache   v3ctrl.ClusterCache
}

func NewTelemetryGatherer(
	rancherVersion string,
	clusterCache v3ctrl.ClusterCache,
	nodeCache v3ctrl.NodeCache,
) TelemetryGatherer {
	return TelemetryGatherer{
		rancherVersion: rancherVersion,
		clusterCache:   clusterCache,
		nodeCache:      nodeCache,
	}
}

func (t *TelemetryGatherer) GetClusterTelemetry() (RancherManagerTelemetry, error) {
	cls, err := t.clusterCache.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	nodeMap := map[ClusterID][]*v3.Node{}
	var localCluster *v3.Cluster
	var localNodes []*v3.Node

	for _, cl := range cls {
		targetNs := cl.Name
		nodePerNs, err := t.nodeCache.List(targetNs, labels.Everything())
		if err != nil {
			return nil, err
		}
		if targetNs == localClusterID {
			localCluster = cl
			localNodes = nodePerNs
			continue
		}
		for _, node := range nodePerNs {
			clName := ClusterID(node.ObjClusterName())
			if _, ok := nodeMap[clName]; !ok {
				nodeMap[clName] = []*v3.Node{}
			}
			nodeMap[clName] = append(nodeMap[clName], node)
		}
	}
	return newTelemetryImpl(t.rancherVersion, localCluster, localNodes, cls, nodeMap), nil
}

func newTelemetryImpl(
	version string,
	localCluster *v3.Cluster,
	localNodes []*v3.Node,
	clList []*v3.Cluster,
	nodeMap map[ClusterID][]*v3.Node,
) *rancherTelemetryImpl {
	return &rancherTelemetryImpl{
		rancherVersion:  version,
		localCluster:    localCluster,
		localNodes:      localNodes,
		managedClusters: clList,
		managedNodes:    nodeMap,
	}
}

func handleCpuCores(cpuQ resource.Quantity) (int, error) {
	cores, ok := cpuQ.AsInt64()
	if !ok {
		return 0, ErrCpuCoresFormat
	}
	return int(cores), nil
}

func handleMemBytes(memQ resource.Quantity) (int, error) {
	memDec := memQ.AsDec()
	if memDec == nil {
		return 0, ErrMemBytesFormat
	}
	bytes, ok := memDec.Unscaled()
	if !ok {
		return 0, ErrMemBytesFormat
	}
	return int(bytes), nil
}
