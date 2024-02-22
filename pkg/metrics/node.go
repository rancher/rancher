package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	nodeProviderLabel  = "provider"
	nodeClusterIDLabel = "cluster_id"
	nodeIsActiveLabel  = "is_active"
)

var (
	nodeLabels = []string{nodeProviderLabel, nodeClusterIDLabel, nodeIsActiveLabel}
	numNodes   = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cluster_manager",
			Name:      "nodes",
			Help:      "Number of nodes in rancher managed clusters",
		}, nodeLabels,
	)
	numCores = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cluster_manager",
			Name:      "node_cores",
			Help:      "Number of node cores in rancher managed clusters",
		}, nodeLabels,
	)
)

const (
	reportInterval = time.Duration(15 * time.Second)
	logPrefix      = "[prometheus-node-metrics]"
)

type nodeMetrics struct {
	nodeCache    mgmtcontrollers.NodeCache
	clusterCache mgmtcontrollers.ClusterCache
}

func (m *nodeMetrics) collect(ctx context.Context) {
	for range ticker.Context(ctx, reportInterval) {
		logrus.Debugf("%s collecting nodes to report metrics", logPrefix)

		nodes, err := m.nodeCache.List("", labels.Everything())
		if err != nil {
			logrus.Errorf("%s couldn't list v3.Nodes: %v", logPrefix, err)
			continue
		}

		var infos []*nodeInfo
		for _, node := range nodes {
			info, err := m.getNodeInfo(node)
			if err != nil {
				logrus.Debugf("%s could not determine node info: %v", logPrefix, err)
				continue
			}
			infos = append(infos, info)
		}

		setMetrics(infos)
	}

	logrus.Debugf("%s context cancelled, exiting", logPrefix)
}

type nodeLabelValues struct {
	provider  string
	clusterID string
	isActive  bool
}

type nodeInfo struct {
	coreCount   int
	labelValues nodeLabelValues
}

const (
	unknownProvider = "unknown"
)

// getNodeInfo uses the v3.Node object and it's corresponding v3.Cluster object to get metric info for a node
func (m *nodeMetrics) getNodeInfo(node *v3.Node) (*nodeInfo, error) {
	cluster, err := m.clusterCache.Get(node.ObjClusterName())
	if err != nil {
		return nil, fmt.Errorf("error getting cluster %s for node %s, %v", node.ObjClusterName(), node.Name, err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("could not get associated cluster for node: %s/%s", node.Namespace, node.Name)
	}
	if cluster.DeletionTimestamp != nil {
		return nil, fmt.Errorf("associated cluster for node: %s/%s is being deleted", node.Namespace, node.Name)
	}

	// a node is considered to be active if it is ready and it belongs to a ready cluster
	isActive := v32.ClusterConditionReady.IsTrue(cluster) && v32.NodeConditionReady.IsTrue(node)

	provider := cluster.Status.Provider
	if provider == "" {
		provider = unknownProvider
	}

	return &nodeInfo{
		coreCount: getNodeCoreCount(node),
		labelValues: nodeLabelValues{
			clusterID: node.ObjClusterName(),
			provider:  provider,
			isActive:  isActive,
		},
	}, nil
}

func getNodeCoreCount(node *v3.Node) int {
	nodeCap := node.Status.InternalNodeStatus.Capacity
	if nodeCap == nil || nodeCap.Cpu() == nil {
		return 0
	}

	cores, ok := nodeCap.Cpu().AsInt64()
	if !ok {
		return 0
	}

	return int(cores)
}

// setMetrics uses a slice of nodeInfo to set prometheus metrics for nodes and node cores
func setMetrics(infos []*nodeInfo) {
	// count nodes and node cores, bucketed by label values
	nodeGauges := make(map[nodeLabelValues]int, 0)
	nodeCoreGauges := make(map[nodeLabelValues]int, 0)
	for _, info := range infos {
		key := info.labelValues
		_, ok := nodeGauges[key]
		if !ok {
			nodeGauges[key] = 1
		} else {
			nodeGauges[key]++
		}

		cores, ok := nodeCoreGauges[key]
		if !ok {
			nodeCoreGauges[key] = info.coreCount
		} else {
			nodeCoreGauges[key] = cores + info.coreCount
		}
	}

	// there could be leftover metrics from previous calls to this method, so we need to clear
	// the metrics just before we set them for the current node info
	numNodes.Reset()
	numCores.Reset()

	// use the built nodeGagues map to update prometheus metrics for nodes
	for k, v := range nodeGauges {
		l := prometheus.Labels{
			nodeProviderLabel:  k.provider,
			nodeClusterIDLabel: k.clusterID,
			nodeIsActiveLabel:  strconv.FormatBool(k.isActive),
		}
		numNodes.With(l).Set(float64(v))
	}

	// use the build nodeCoreGauges map to update prometheus metrics for node cores
	for k, v := range nodeCoreGauges {
		l := prometheus.Labels{
			nodeProviderLabel:  k.provider,
			nodeClusterIDLabel: k.clusterID,
			nodeIsActiveLabel:  strconv.FormatBool(k.isActive),
		}
		numCores.With(l).Set(float64(v))
	}
}
