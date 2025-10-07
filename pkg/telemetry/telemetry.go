package telemetry

import (
	"errors"
	"iter"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	v3ctrl "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/telemetry/initcond"
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

type RancherVersionTelemetry interface {
	InstallUUID() string
	ClusterUUID() string
	ServerURL() string
	RancherVersion() string
	RancherGitHash() string
	FeatureFlags() []string
}

type RancherManagerTelemetry interface {
	// Management

	ManagedClusterCount() int
	PerManagedClusterTelemetry() iter.Seq2[ClusterID, ClusterTelemetry]

	// Local cluster-specific

	LocalNodeCount() int
	LocalClusterTelemetry() ClusterTelemetry

	// RancherVersionTelemetry exposes versioning related metadata
	RancherVersionTelemetry
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
	gitHash        string
	installUUID    string
	clusterUUID    string
	serverURL      string

	localCluster *v3.Cluster
	localNodes   []*v3.Node

	managedClusters []*v3.Cluster
	managedNodes    map[ClusterID][]*v3.Node
}

var _ RancherManagerTelemetry = (*rancherTelemetryImpl)(nil)

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

func (r *rancherTelemetryImpl) RancherGitHash() string {
	return r.gitHash
}

func (r *rancherTelemetryImpl) InstallUUID() string {
	return r.installUUID
}

func (r *rancherTelemetryImpl) ClusterUUID() string {
	return r.clusterUUID
}

func (r *rancherTelemetryImpl) ServerURL() string {
	return r.serverURL
}

func (r *rancherTelemetryImpl) FeatureFlags() []string {
	return features.ListEnabled()
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
	gitHash        string
	installUUID    string
	clusterUUID    string
	serverURL      string

	nodeCache    v3ctrl.NodeCache
	clusterCache v3ctrl.ClusterCache
}

func NewTelemetryGatherer(
	clusterCache v3ctrl.ClusterCache,
	nodeCache v3ctrl.NodeCache,
) TelemetryGatherer {
	return TelemetryGatherer{
		clusterCache: clusterCache,
		nodeCache:    nodeCache,
	}
}

func (t *TelemetryGatherer) visitWithInitInfo(info initcond.InitInfo) {
	t.clusterUUID = info.ClusterUUID
	t.serverURL = info.ServerURL
	t.installUUID = info.InstallUUID
	t.rancherVersion = info.RancherVersion
	t.gitHash = info.GitHash
}

func (t *TelemetryGatherer) GetClusterTelemetry() (RancherManagerTelemetry, error) {
	cls, err := t.clusterCache.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	nodeMap := map[ClusterID][]*v3.Node{}
	var localCluster *v3.Cluster
	var localNodes []*v3.Node

	managedCls := make([]*v3.Cluster, len(cls)-1)
	i := 0
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
		managedCls[i] = cl
		i++
		for _, node := range nodePerNs {
			clName := ClusterID(node.ObjClusterName())
			if _, ok := nodeMap[clName]; !ok {
				nodeMap[clName] = []*v3.Node{}
			}
			nodeMap[clName] = append(nodeMap[clName], node)
		}
	}
	return newTelemetryImpl(
		t.rancherVersion,
		t.gitHash,
		t.installUUID,
		t.clusterUUID,
		t.serverURL,
		localCluster,
		localNodes,
		managedCls,
		nodeMap,
	), nil
}

func newTelemetryImpl(
	version,
	gitHash,
	installUUID,
	clusterUUID string,
	serverURL string,
	localCluster *v3.Cluster,
	localNodes []*v3.Node,
	clList []*v3.Cluster,
	nodeMap map[ClusterID][]*v3.Node,
) *rancherTelemetryImpl {
	return &rancherTelemetryImpl{
		rancherVersion:  version,
		gitHash:         gitHash,
		installUUID:     installUUID,
		clusterUUID:     clusterUUID,
		serverURL:       serverURL,
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
