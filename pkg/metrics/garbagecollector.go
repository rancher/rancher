package metrics

import (
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/rancher/rancher/pkg/settings"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	rm "github.com/rancher/remotedialer/metrics"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	targetMetricsByNameForClientKey = []interface{}{
		rm.TotalAddWS, rm.TotalRemoveWS, rm.TotalAddConnectionsForWS, rm.TotalRemoveConnectionsForWS,
		rm.TotalTransmitBytesOnWS, rm.TotalTransmitErrorBytesOnWS, rm.TotalReceiveBytesOnWS,
	}

	targetMetricsByIPForPeer = []interface{}{
		rm.TotalAddPeerAttempt, rm.TotalPeerConnected, rm.TotalPeerDisConnected,
	}
)

type metricGarbageCollector struct {
	clusterLister  v3.ClusterLister
	nodeLister     v3.NodeLister
	endpointLister v1.EndpointsLister
}

func (gc *metricGarbageCollector) metricGarbageCollection() {
	logrus.Debugf("[metrics-garbage-collector] Start")

	isClusterMode := settings.Namespace.Get() != "" && settings.PeerServices.Get() != ""

	// Existing resources
	observedResourceNames := map[string]bool{}
	// Exisiting Metrics Labels
	observedLabelsMap := map[string]map[interface{}][]map[string]string{}

	// Get Clusters
	clusters, err := gc.clusterLister.List("", labels.Everything())
	if err != nil {
		logrus.Errorf("[metrics-garbage-collector] failed to list clusters: %s", err)
		return
	}
	for _, cluster := range clusters {
		if _, ok := observedResourceNames[cluster.Name]; !ok {
			observedResourceNames[cluster.Name] = true
		}
	}
	// Get Nodes
	nodes, err := gc.nodeLister.List("", labels.Everything())
	if err != nil {
		logrus.Errorf("[metrics-garbage-collector] failed to list nodes: %s", err)
	}
	for _, node := range nodes {
		if _, ok := observedResourceNames[node.Namespace+":"+node.Name]; !ok {
			observedResourceNames[node.Namespace+":"+node.Name] = true
		}
	}
	// Get Endpoints
	if isClusterMode {
		endpoints, err := gc.endpointLister.List(settings.Namespace.Get(), labels.Everything())
		if err != nil {
			logrus.Errorf("[metrics-garbage-collector] failed to list endpoints: %s", err)
		}
		for _, svc := range strings.Split(settings.PeerServices.Get(), ",") {
			for _, e := range endpoints {
				// If Endpoint is associated with PeerServices service
				if e.Name == strings.TrimSpace(svc) {
					for _, subset := range e.Subsets {
						for _, addr := range subset.Addresses {
							observedResourceNames[addr.IP] = true
						}
					}
				}
			}
		}
	}

	buildObservedLabelMaps(targetMetricsByNameForClientKey, "clientkey", observedLabelsMap)
	buildObservedLabelMaps(targetMetricsByIPForPeer, "peer", observedLabelsMap)
	buildObservedLabelMaps([]interface{}{clusterOwner}, "cluster", observedLabelsMap)

	removedCount := removeMetricsForDeletedResource(observedLabelsMap, observedResourceNames)

	logrus.Debugf("[metrics-garbage-collector] Finished - removed %d items", removedCount)
}

func buildObservedLabelMaps(collectors []interface{}, targetLabel string, observedLabels map[string]map[interface{}][]map[string]string) int {
	// Example of the map structure of observedLabels:
	// {
	//   "c-fz6fq": {
	//      collectorA: [ {"clientkey": "c-fz6fq", "peer": "false"}, ],
	//      collectorB: [ {"clientkey": "c-fz6fq", "peer": "false"}, ],
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

func removeMetricsForDeletedResource(observedMetrics map[string]map[interface{}][]map[string]string, observedResources map[string]bool) int {
	removedCount := 0
	for m, collectors := range observedMetrics {
		// resource still exists
		if _, ok := observedResources[m]; ok {
			continue
		}
		logrus.Infof("[metrics-garbage-collector] remove metrics related to %s", m)
		// resource doesn't exist, delete all related metrics
		for collector, labels := range collectors {
			for _, label := range labels {
				switch v := collector.(type) {
				case *prometheus.CounterVec:
					if v.Delete(label) {
						removedCount++
					} else {
						logrus.Errorf("[metrics-garbage-collector] failed to delete %T metrics related to %s: %v", v, m, label)
					}
				case *prometheus.GaugeVec:
					if v.Delete(label) {
						removedCount++
					} else {
						logrus.Errorf("[metrics-garbage-collector] failed to delete %T metrics related to %s: %v", v, m, label)
					}
				default:
					logrus.Errorf("[metrics-garbage-collector] saw unknown Metric definition %T", v)
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
			break
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
