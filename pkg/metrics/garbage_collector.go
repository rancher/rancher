package metrics

import (
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	nm "github.com/rancher/norman/metrics"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	GCTargetMetricsByNameForClientKey = []interface{}{
		nm.TotalAddWS, nm.TotalRemoveWS, nm.TotalAddConnectionsForWS, nm.TotalRemoveConnectionsForWS,
		nm.TotalTransmitBytesOnWS, nm.TotalTransmitErrorBytesOnWS, nm.TotalReceiveBytesOnWS,
	}

	GCTargetMetricsByIPForPeer = []interface{}{
		nm.TotalAddPeerAttempt, nm.TotalPeerConnected, nm.TotalPeerDisConnected,
	}
)

func MetricGarbageCollection(context *config.ScaledContext) {
	logrus.Debugf("MetricGarbageCollector Start")

	clusterLister := context.Management.Clusters("").Controller().Lister()
	nodeLister := context.Management.Nodes("").Controller().Lister()
	endpointLister := context.Core.Endpoints(settings.Namespace.Get()).Controller().Lister()
	isClusterMode := settings.Namespace.Get() != "" && settings.PeerServices.Get() != ""

	// Existing resources
	observedResourceNames := map[string]bool{}
	// Exisiting Metrics Labels
	observedLabelsMap := map[string]map[interface{}][]map[string]string{}

	// Get Clusters
	clusters, err := clusterLister.List("", labels.Everything())
	if err != nil {
		logrus.Errorf("MetricGarbageCollector failed to list clusters: %s", err)
		return
	}
	for _, cluster := range clusters {
		if _, ok := observedResourceNames[cluster.Name]; !ok {
			observedResourceNames[cluster.Name] = true
		}
	}
	// Get Nodes
	nodes, err := nodeLister.List("", labels.Everything())
	if err != nil {
		logrus.Errorf("MetricGarbageCollector failed to list nodes: %s", err)
	}
	for _, node := range nodes {
		if _, ok := observedResourceNames[node.Namespace+":"+node.Name]; !ok {
			observedResourceNames[node.Namespace+":"+node.Name] = true
		}
	}
	// Get Endpoints
	endpoints, err := endpointLister.List(settings.Namespace.Get(), labels.Everything())
	countOfRancherServer := 0
	if isClusterMode {
		for _, svc := range strings.Split(settings.PeerServices.Get(), ",") {
			for _, e := range endpoints {
				// If Endpoint is associated with PeerServices service
				if e.Name == strings.TrimSpace(svc) {
					for _, subset := range e.Subsets {
						for _, addr := range subset.Addresses {
							observedResourceNames[addr.IP] = true
							countOfRancherServer++
						}
					}
				}
			}
		}
	}
	logrus.Debugf("MetricGarbageCollector saw %d clusters", len(clusters))
	logrus.Debugf("MetricGarbageCollector saw %d nodes", len(nodes))
	logrus.Debugf("MetricGarbageCollector saw %d rancher servers", countOfRancherServer)

	observedClientKey := BuildObservedLabelMaps(GCTargetMetricsByNameForClientKey, "clientkey", observedLabelsMap)
	observedPeer := BuildObservedLabelMaps(GCTargetMetricsByIPForPeer, "peer", observedLabelsMap)
	observedCluster := BuildObservedLabelMaps([]interface{}{ClusterOwner}, "cluster", observedLabelsMap)
	logrus.Debugf("MetricGarbageCollector saw %d clientkey of metrics", observedClientKey)
	logrus.Debugf("MetricGarbageCollector saw %d peer of metrics", observedPeer)
	logrus.Debugf("MetricGarbageCollector saw %d cluster of metrics", observedCluster)

	removedCount := RemoveMetricsForDeletedResource(observedLabelsMap, observedResourceNames)
	logrus.Debugf("MetricGarbageCollector removed %d metrics", removedCount)

	logrus.Debugf("MetricGarbageCollector Finished")
}

func BuildObservedLabelMaps(collectors []interface{}, targetLabel string, observedLabels map[string]map[interface{}][]map[string]string) int {
	// {
	//   "c-fz6fq": {
	//      collectorA: [ {"clientkey": "c-fz6fq", "peer": "fales"}, ],
	//      collectorB: [ {"clientkey": "c-fz6fq", "peer": "fales"}, ],
	//   },
	//   "<targetLabel value>": {
	//   }
	// }
	count := 0
	for _, collector := range collectors {
		metricChan := make(chan prometheus.Metric)
		metricFrame := &dto.Metric{}
		go func() { collector.(prometheus.Collector).Collect(metricChan); close(metricChan) }()
		for metric := range metricChan {
			metric.Write(metricFrame)
			for _, label := range metricFrame.Label {
				if label.GetName() == targetLabel {
					// Initialize data structure
					if observedLabels[label.GetValue()] == nil {
						newCollectorMap := map[interface{}][]map[string]string{}
						newLabelList := []map[string]string{}
						observedLabels[label.GetValue()] = newCollectorMap
						observedLabels[label.GetValue()][collector] = newLabelList
					}
					metricLabelMap := metrcisLabelToMap(metricFrame)
					newLabels := appendIfLabelIsNotInList(metricLabelMap, observedLabels[label.GetValue()][collector])
					observedLabels[label.GetValue()][collector] = newLabels
					count++
				}
			}
		}
	}
	return count
}

func RemoveMetricsForDeletedResource(observedMetrics map[string]map[interface{}][]map[string]string, observedResources map[string]bool) int {
	removedCount := 0
	for m, collectors := range observedMetrics {
		// resource still exists
		if _, ok := observedResources[m]; ok {
			continue
		}
		// resource doesn't exist, delete all related metrics
		for collector, labels := range collectors {
			for _, label := range labels {
				switch v := collector.(type) {
				case *prometheus.CounterVec:
					if v.Delete(label) {
						removedCount++
						logrus.Infof(
							"MetricGarbageCollector remove %T metrics related to %s: %v", v, m, label)
					} else {
						logrus.Errorf(
							"MetricGarbageCollector failed to delete %T metrics related to %s: %v", v, m, label)
					}
				case *prometheus.GaugeVec:
					if v.Delete(label) {
						removedCount++
						logrus.Infof(
							"MetricGarbageCollector remove %T metrics related to %s: %v", v, m, label)
					} else {
						logrus.Errorf(
							"MetricGarbageCollector failed to delete %T metrics related to %s: %v", v, m, label)
					}
				default:
					logrus.Errorf("MetricGarbageCollector saw unknown Metric definition %T", v)
				}
			}

		}
	}
	return removedCount
}

func appendIfLabelIsNotInList(targetLabel map[string]string, labelList []map[string]string) []map[string]string {
	found := false
	for _, label := range labelList {
		if reflect.DeepEqual(targetLabel, label) {
			found = true
		}
	}
	if !found {
		labelList = append(labelList, targetLabel)
	}
	return labelList
}

func metrcisLabelToMap(m *dto.Metric) map[string]string {
	result := map[string]string{}
	for _, label := range m.Label {
		result[label.GetName()] = label.GetValue()
	}
	return result
}
