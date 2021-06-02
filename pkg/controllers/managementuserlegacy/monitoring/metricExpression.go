package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/ghodss/yaml"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

var (
	preDefinedClusterMetrics = getPredefinedClusterMetrics()
	preDefinedClusterGraph   = getPredefinedClusterGraph()
	preDefinedProjectGraph   = getPredefinedProjectGraph()
)

type templateData struct {
	ProjectName string
}

func getPredefinedClusterMetrics() []*managementv3.MonitorMetric {
	yamls := strings.Split(MonitorMetricsTemplate, "\n---\n")
	var rtn []*managementv3.MonitorMetric
	for _, yml := range yamls {
		var tmp managementv3.MonitorMetric
		if err := yamlToObject(yml, &tmp); err != nil {
			panic(err)
		}
		if tmp.Name == "" {
			continue
		}
		rtn = append(rtn, &tmp)
	}

	return rtn
}

func getPredefinedClusterGraph() []*managementv3.ClusterMonitorGraph {
	yamls := strings.Split(ClusterMetricExpression, "\n---\n")
	var rtn []*managementv3.ClusterMonitorGraph
	for _, yml := range yamls {
		var tmp managementv3.ClusterMonitorGraph
		if err := yamlToObject(yml, &tmp); err != nil {
			panic(err)
		}
		if tmp.Name == "" {
			continue
		}
		rtn = append(rtn, &tmp)
	}

	return rtn
}

func getPredefinedProjectGraph() []*managementv3.ProjectMonitorGraph {
	yamls := strings.Split(ProjectMetricExpression, "\n---\n")
	var rtn []*managementv3.ProjectMonitorGraph
	for _, yml := range yamls {
		var tmp managementv3.ProjectMonitorGraph
		if err := yamlToObject(yml, &tmp); err != nil {
			panic(err)
		}
		if tmp.Name == "" {
			continue
		}
		rtn = append(rtn, &tmp)
	}

	return rtn
}

func yamlToObject(yml string, obj interface{}) error {
	jsondata, err := yaml.YAMLToJSON([]byte(yml))
	if err != nil {
		return err
	}
	return json.Unmarshal(jsondata, obj)
}

func generate(text string, data templateData) (string, error) {
	t, err := template.New("expression-yaml").Parse(text)
	if err != nil {
		return "", fmt.Errorf("parse template expression-yaml failed, %v", err)
	}

	var contentBuf bytes.Buffer
	err = t.Execute(&contentBuf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template expression-yaml, %v", err)
	}

	return contentBuf.String(), nil
}

var (
	ClusterMetricExpression = `
---
# Source: metric-expression-cluster/templates/graphapiserver.yaml
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: apiserver
    cluster-graph: kube-component
  name: apiserver-request-latency
spec:
  resourceType: apiserver
  displayResourceType: kube-component
  priority: 300
  title: apiserver-request-latency
  detailsMetricsSelector:
    component: apiserver
    details: "true"
    metric: request-latency-milliseconds-avg
  metricsSelector:
    details: "false"
    component: apiserver
    metric: request-latency-milliseconds-avg
  yAxis:
    unit: ms
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: apiserver
    cluster-graph: kube-component
  name: apiserver-request-count
spec:
  resourceType: apiserver
  displayResourceType: kube-component
  priority: 301
  title: apiserver-request-count
  detailsMetricsSelector:
    component: apiserver
    details: "true"
    metric: request-count-sum-rate
  metricsSelector:
    details: "false"
    component: apiserver
    metric: request-count-sum-rate
  yAxis:
    unit: number
---
# Source: metric-expression-cluster/templates/graphcluster.yaml
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-cpu-usage
spec:
  resourceType: cluster
  priority: 100
  title: cluster-cpu-usage
  metricsSelector:
    details: "false"
    component: cluster
    metric: cpu-usage-seconds-sum-rate
  detailsMetricsSelector:
    details: "true"
    component: cluster
    metric: cpu-usage-seconds-sum-rate
  yAxis:
    unit: percent
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-cpu-load
spec:
  resourceType: cluster
  priority: 101
  title: cluster-cpu-load
  metricsSelector:
    details: "false"
    component: cluster
    graph: cpu-load
  detailsMetricsSelector:
    details: "true"
    component: cluster
    graph: cpu-load
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-memory-usage
spec:
  resourceType: cluster
  priority: 102
  title: cluster-memory-usage
  metricsSelector:
    details: "false"
    component: cluster
    metric: memory-usage-percent
  detailsMetricsSelector:
    details: "true"
    component: cluster
    metric: memory-usage-percent
  yAxis:
    unit: percent
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-fs-usage-percent
spec:
  resourceType: cluster
  priority: 103
  title: cluster-fs-usage-percent
  thresholds: 10
  metricsSelector:
    details: "false"
    component: cluster
    metric: fs-usage-percent
  detailsMetricsSelector:
    details: "true"
    component: cluster
    metric: fs-usage-percent
  yAxis:
    unit: percent
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-disk-io
spec:
  resourceType: cluster
  priority: 104
  title: cluster-disk-io
  thresholds: 10
  metricsSelector:
    details: "false"
    component: cluster
    graph: disk-io
  detailsMetricsSelector:
    details: "true"
    component: cluster
    graph: disk-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-network-io
spec:
  resourceType: cluster
  priority: 105
  title: cluster-network-io
  thresholds: 10
  metricsSelector:
    details: "false"
    component: cluster
    graph: network-io
  detailsMetricsSelector:
    details: "true"
    component: cluster
    graph: network-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: cluster
  name: cluster-network-packet
spec:
  resourceType: cluster
  priority: 106
  title: cluster-network-packet
  thresholds: 10
  metricsSelector:
    details: "false"
    component: cluster
    graph: network-packet
  detailsMetricsSelector:
    details: "true"
    component: cluster
    graph: network-packet
  yAxis:
    unit: pps
---
# Source: metric-expression-cluster/templates/graphcontrollermanager.yaml
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: controllermanager
    cluster-graph: kube-component
  name: controllermanager-queue-depth
spec:
  resourceType: controllermanager
  displayResourceType: kube-component
  priority: 310
  title: controllermanager-queue-depth
  metricsSelector:
    details: "false"
    component: controllermanager
  detailsMetricsSelector:
    details: "true"
    component: controllermanager
  yAxis:
    unit: number

---
# Source: metric-expression-cluster/templates/graphetcd.yaml
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-server-leader-sum
spec:
  resourceType: etcd
  priority: 200
  title: etcd-server-leader-sum
  description: etcd server leader sum
  metricsSelector:
    details: "false"
    component: etcd
    metric: server-leader-sum
  detailsMetricsSelector:
    details: "true"
    component: etcd
    metric: server-leader-sum
  yAxis:
    unit: number
  graphType: singlestat
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-server-failed-proposal
spec:
  resourceType: etcd
  priority: 201
  title: etcd-server-failed-proposal
  description: etcd server failed proposal
  metricsSelector:
    details: "false"
    component: etcd
    metric: server-failed-proposal
  detailsMetricsSelector:
    details: "true"
    component: etcd
    metric: server-failed-proposal
  yAxis:
    unit: number
  graphType: singlestat
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-leader-change
spec:
  resourceType: etcd
  priority: 202
  title: etcd-leader-change
  description: etcd leader change
  metricsSelector:
    details: "false"
    component: etcd
    metric: server-leader-changes-seen-sum-increase
  detailsMetricsSelector:
    details: "true"
    component: etcd
    metric: server-leader-changes-seen-sum-increase
  yAxis:
    unit: number
  graphType: singlestat
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-grpc-client
spec:
  resourceType: etcd
  priority: 203
  title: etcd-grpc-client
  description: etcd grpc client receive/send bytes sum rate
  metricsSelector:
    details: "false"
    component: etcd
    graph: rpc-client-traffic
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: rpc-client-traffic
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
    metric: db-bytes-sum 
  name: etcd-db-bytes-sum
spec:
  resourceType: etcd
  priority: 204
  title: etcd-db-bytes-sum
  description: etcd db bytes sum
  metricsSelector:
    details: "false"
    component: etcd
    metric: db-bytes-sum 
  detailsMetricsSelector:
    details: "true"
    component: etcd
    metric: db-bytes-sum 
  yAxis:
    unit: byte
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-stream
spec:
  resourceType: etcd
  priority: 205
  title: etcd-stream
  description: Etcd lease/watch stream
  metricsSelector:
    details: "false"
    component: etcd
    graph: etcd-stream
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: etcd-stream
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-peer-traffic
spec:
  resourceType: etcd
  priority: 206
  title: etcd-peer-traffic
  description: Etcd peer traffic in/out
  metricsSelector:
    details: "false"
    component: etcd
    graph: etcd-peer-traffic
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: etcd-peer-traffic
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-raft-proposals
spec:
  resourceType: etcd
  priority: 207
  title: etcd-raft-proposals
  description: Etcd raft proposals
  metricsSelector:
    details: "false"
    component: etcd
    graph: proposal
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: proposal
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-rpc-rate
spec:
  resourceType: etcd
  priority: 208
  title: etcd-rpc-rate
  description: Etcd rpc-rate
  metricsSelector:
    details: "false"
    component: etcd
    graph: rpc-rate
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: rpc-rate
  yAxis:
    unit: ops
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-disk-operate
spec:
  resourceType: etcd
  priority: 209
  title: etcd-disk-operate
  description: Etcd disk operate
  metricsSelector:
    details: "false"
    component: etcd
    graph: disk-operate
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: disk-operate
  yAxis:
    unit: seconds
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: etcd
  name: etcd-sync-duration
spec:
  resourceType: etcd
  priority: 209
  title: etcd-sync-duration
  description: Etcd sync-duration
  metricsSelector:
    details: "false"
    component: etcd
    graph: sync-duration
  detailsMetricsSelector:
    details: "true"
    component: etcd
    graph: sync-duration
  yAxis:
    unit: seconds
---
# Source: metric-expression-cluster/templates/graphfluentd.yaml
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: fluentd
    cluster-graph: rancher-component
  name: fluentd-input-record-number
spec:
  resourceType: fluentd
  displayResourceType: rancher-component
  priority: 300
  title: fluentd-input-record-number
  metricsSelector:
    details: "false"
    component: fluentd
    metric: input-record
  detailsMetricsSelector:
    details: "true"
    component: fluentd
    metric: input-record
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: fluentd
    cluster-graph: rancher-component
  name: fluentd-output-record-number
spec:
  resourceType: fluentd
  displayResourceType: rancher-component
  priority: 301
  title: fluentd-output-record-number
  metricsSelector:
    details: "false"
    component: fluentd
    metric: output-record
  detailsMetricsSelector:
    details: "true"
    component: fluentd
    metric: output-record
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: fluentd
    cluster-graph: rancher-component
  name: fluentd-output-errors
spec:
  resourceType: fluentd
  displayResourceType: rancher-component
  priority: 301
  title: fluentd-output-errors
  metricsSelector:
    details: "false"
    component: fluentd
    metric: output-errors
  detailsMetricsSelector:
    details: "true"
    component: fluentd
    metric: output-errors
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: fluentd
    cluster-graph: rancher-component
  name: fluentd-buffer-queue-length
spec:
  resourceType: fluentd
  displayResourceType: rancher-component
  priority: 301
  title: fluentd-buffer-queue-length
  metricsSelector:
    details: "false"
    component: fluentd
    metric: buffer-queue-length
  detailsMetricsSelector:
    details: "true"
    component: fluentd
    metric: buffer-queue-length
  yAxis:
    unit: number
---
# Source: metric-expression-cluster/templates/graphingresscontroller.yaml
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: ingresscontroller
    cluster-graph: kube-component
  name: ingresscontroller-nginx-connection
spec:
  resourceType: ingresscontroller
  displayResourceType: kube-component
  priority: 330
  title: ingresscontroller-nginx-connection
  metricsSelector:
    details: "false"
    component: ingresscontroller
    graph: nginx-connection
  detailsMetricsSelector:
    details: "true"
    component: ingresscontroller
    graph: nginx-connection
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: ingresscontroller
    cluster-graph: kube-component
  name: ingresscontroller-request-process-time
spec:
  resourceType: ingresscontroller
  displayResourceType: kube-component
  priority: 331
  title: ingresscontroller-request-process-time
  metricsSelector:
    details: "false"
    component: ingresscontroller
    metric: request-process-seconds
  detailsMetricsSelector:
    details: "true"
    component: ingresscontroller
    metric: request-process-seconds
  yAxis:
    unit: seconds
---
# Source: metric-expression-cluster/templates/graphnode.yaml
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-cpu-usage
spec:
  resourceType: node
  priority: 500
  title: node-cpu-usage
  metricsSelector:
    details: "false"
    component: node
    metric: cpu-usage-seconds-sum-rate
  detailsMetricsSelector:
    details: "true"
    component: node
    metric: cpu-usage-seconds-sum-rate
  yAxis:
    unit: percent
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-cpu-load
spec:
  resourceType: node
  priority: 501
  title: node-cpu-load
  metricsSelector:
    details: "false"
    component: node
    graph: cpu-load
  detailsMetricsSelector:
    details: "true"
    component: node
    graph: cpu-load
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-memory-usage
spec:
  resourceType: node
  priority: 502
  title: node-memory-usage
  metricsSelector:
    details: "false"
    component: node
    metric: memory-usage-percent
  detailsMetricsSelector:
    details: "true"
    component: node
    metric: memory-usage-percent
  yAxis:
    unit: percent
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-fs-usage-percent
spec:
  resourceType: node
  priority: 503
  title: node-fs-usage-percent
  thresholds: 10
  metricsSelector:
    details: "false"
    component: node
    metric: fs-usage-percent
  detailsMetricsSelector:
    details: "true"
    component: node
    metric: fs-usage-percent
  yAxis:
    unit: percent
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-disk-io
spec:
  resourceType: node
  priority: 504
  title: node-disk-io
  thresholds: 10
  metricsSelector:
    details: "false"
    component: node
    graph: disk-io
  detailsMetricsSelector:
    details: "true"
    component: node
    graph: disk-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-network-io
spec:
  resourceType: node
  priority: 505
  title: node-network-io
  thresholds: 10
  metricsSelector:
    details: "false"
    component: node
    graph: network-io
  detailsMetricsSelector:
    details: "true"
    component: node
    graph: network-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: node
  name: node-network-packet
spec:
  resourceType: node
  priority: 506
  title: node-network-packet
  thresholds: 10
  metricsSelector:
    details: "false"
    component: node
    graph: network-packet
  detailsMetricsSelector:
    details: "true"
    component: node
    graph: network-packet
  yAxis:
    unit: pps
---
# Source: metric-expression-cluster/templates/graphscheduler.yaml
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: scheduler
    cluster-graph: kube-component
  name: scheduler-e-2-e-scheduling-latency-seconds-quantile
spec:
  resourceType: scheduler
  displayResourceType: kube-component
  priority: 320
  title: scheduler-e-2-e-scheduling-latency-seconds-quantile
  thresholds: 10
  metricsSelector:
    details: "false"
    component: scheduler
    metric: e-2-e-scheduling-latency-seconds-quantile
  detailsMetricsSelector:
    details: "true"
    component: scheduler
    metric: e-2-e-scheduling-latency-seconds-quantile
  yAxis:
    unit: seconds
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: scheduler
    cluster-graph: kube-component
  name: scheduler-total-preemption-attempts
spec:
  resourceType: scheduler
  displayResourceType: kube-component
  priority: 321
  title: scheduler-total-preemption-attempts
  thresholds: 10
  metricsSelector:
    details: "false"
    component: scheduler
    metric: total-preemption-attempts
  detailsMetricsSelector:
    details: "true"
    component: scheduler
    metric: total-preemption-attempts
  yAxis:
    unit: number
---
apiVersion: management.cattle.io/v3
kind: ClusterMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: cluster
    component: scheduler
    cluster-graph: kube-component
  name: scheduler-pod-unscheduler
spec:
  resourceType: scheduler
  displayResourceType: kube-component
  priority: 322
  title: scheduler-pod-unscheduler
  thresholds: 10
  metricsSelector:
    details: "false"
    component: scheduler
    metric: pod-unscheduler
  detailsMetricsSelector:
    details: "true"
    component: scheduler
    metric: pod-unscheduler
  yAxis:
    unit: number
---
`
	MonitorMetricsTemplate = `
---
# Source: metric-expression-cluster/templates/expressionapiserver.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: apiserver-request-latency-milliseconds-avg
  labels:
    app: metric-expression
    component: apiserver
    details: "false"
    level: cluster
    metric: request-latency-milliseconds-avg
    source: rancher-monitoring
spec:
  expression: avg(apiserver_request_latencies_sum / apiserver_request_latencies_count)
    by (instance) /1e+06
  legendFormat: '[[instance]]'
  description: apiserver request latency milliseconds avg
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: apiserver-request-latency-milliseconds-avg-details
  labels:
    app: metric-expression
    component: apiserver
    details: "true"
    level: cluster
    metric: request-latency-milliseconds-avg
    source: rancher-monitoring
spec:
  expression: avg(apiserver_request_latencies_sum / apiserver_request_latencies_count)
    by (instance, verb) /1e+06
  legendFormat: '[[verb]]([[instance]])'
  description: apiserver request latency milliseconds avg
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: apiserver-request-count-sum-rate
  labels:
    app: metric-expression
    component: apiserver
    details: "false"
    graph: request-count
    level: cluster
    metric: request-count-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(apiserver_request_count[5m])) by (instance)
  legendFormat: '[[instance]]'
  description: apiserver request count sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: apiserver-request-count-sum-rate-details
  labels:
    app: metric-expression
    component: apiserver
    details: "true"
    graph: request-count
    level: cluster
    metric: request-count-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(apiserver_request_count[5m])) by (instance,
    code)
  legendFormat: '[[code]]([[instance]])'
  description: apiserver request count sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: apiserver-request-error-count-sum-rate
  labels:
    app: metric-expression
    component: apiserver
    details: "false"
    graph: request-count
    level: cluster
    metric: request-error-count-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(apiserver_request_count{instance=~"$instance", code!~"2.."}[5m]))
    by (instance)
  legendFormat: '[[instance]]'
  description: apiserver request error count sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: apiserver-request-error-count-sum-rate-details
  labels:
    app: metric-expression
    component: apiserver
    details: "true"
    graph: request-count
    level: cluster
    metric: request-error-count-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(apiserver_request_count{instance=~"$instance", code!~"2.."}[5m]))
    by (instance, code)
  legendFormat: '[[code]]([[instance]])'
  description: apiserver request error count sum rate
---

---
# Source: metric-expression-cluster/templates/expressioncluster.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-disk-io-reads-bytes-sum-rate
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: disk-io
    level: cluster
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_read_bytes_total[5m]))
  legendFormat: Read
  description: cluster disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-disk-io-reads-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: disk-io
    level: cluster
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_read_bytes_total[5m])) by
    (instance)
  legendFormat: Read([[instance]])
  description: cluster disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-bytes-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-io
    level: cluster
    metric: network-transmit-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Transmit
  description: cluster network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-bytes-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-io
    level: cluster
    metric: network-transmit-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Transmit([[instance]])
  description: cluster network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-load-5
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: cpu-load
    level: cluster
    metric: cpu-load-5
    source: rancher-monitoring
spec:
  expression: sum(node_load5) by (instance)
  legendFormat: Load5([[instance]])
  description: cluster cpu load 5
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-load-5-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: cpu-load
    level: cluster
    metric: cpu-load-5
    source: rancher-monitoring
spec:
  expression: sum(node_load5) by (instance)
  legendFormat: Load5([[instance]])
  description: cluster cpu load 5
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-load-1
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: cpu-load
    level: cluster
    metric: cpu-load-1
    source: rancher-monitoring
spec:
  expression: sum(node_load1) by (instance)
  legendFormat: Load1([[instance]])
  description: cluster cpu load 1
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-load-1-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: cpu-load
    level: cluster
    metric: cpu-load-1
    source: rancher-monitoring
spec:
  expression: sum(node_load1) by (instance)
  legendFormat: Load1([[instance]])
  description: cluster cpu load 1
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-disk-io-writes-bytes-sum-rate
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: disk-io
    level: cluster
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_written_bytes_total[5m]))
  legendFormat: Write
  description: cluster disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-disk-io-writes-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: disk-io
    level: cluster
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_written_bytes_total[5m]))
    by (instance)
  legendFormat: Write([[instance]])
  description: cluster disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-fs-usage-percent
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    level: cluster
    metric: fs-usage-percent
    source: rancher-monitoring
spec:
  expression: (sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+"})
     - sum(node_filesystem_free_bytes{device!~"rootfs|HarddiskVolume.+"})
    ) / sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+"})
  legendFormat: Disk usage
  description: cluster fs usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-fs-usage-percent-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    level: cluster
    metric: fs-usage-percent
    source: rancher-monitoring
spec:
  expression: (sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+"})
    by (instance) - sum(node_filesystem_free_bytes{device!~"rootfs|HarddiskVolume.+"})
    by (instance)) / sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+"})
    by (instance)
  legendFormat: '[[instance]]'
  description: cluster fs usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-errors-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-receive-errors-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Receive errors
  description: cluster network receive errors sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-errors-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-receive-errors-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Receive errors([[instance]])
  description: cluster network receive errors sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-load-15
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: cpu-load
    level: cluster
    metric: cpu-load-15
    source: rancher-monitoring
spec:
  expression: sum(node_load15) by (instance) 
  legendFormat: Load15([[instance]])
  description: cluster cpu load 15
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-load-15-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: cpu-load
    level: cluster
    metric: cpu-load-15
    source: rancher-monitoring
spec:
  expression: sum(node_load15) by (instance)
  legendFormat: Load15([[instance]])
  description: cluster cpu load 15
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-bytes-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-io
    level: cluster
    metric: network-receive-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Receive
  description: cluster network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-bytes-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-io
    level: cluster
    metric: network-receive-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Receive([[instance]])
  description: cluster network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-packets-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Receive packets
  description: cluster network receive packets sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-packets-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Receive packets([[instance]])
  description: cluster network receive packets sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-errors-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-transmit-errors-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Transmit errors
  description: cluster network transmit errors sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-errors-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-transmit-errors-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Transmit errors([[instance]])
  description: cluster network transmit errors sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-packets-dropped-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-dropped-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Receive dropped
  description: cluster network receive packets dropped sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-receive-packets-dropped-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-dropped-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Receive dropped([[instance]])
  description: cluster network receive packets dropped sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-packets-dropped-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-dropped-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Transmit dropped
  description: cluster network transmit packets dropped sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-packets-dropped-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-dropped-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Transmit dropped([[instance]])
  description: cluster network transmit packets dropped sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-packets-sum
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
  legendFormat: Transmit packets
  description: cluster network transmit packets sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-network-transmit-packets-sum-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-sum
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*"}[5m]))
    by (instance)
  legendFormat: Transmit packets([[instance]])
  description: cluster network transmit packets sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-usage-seconds-sum-rate
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    level: cluster
    metric: cpu-usage-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: 1 - (avg(irate(node_cpu_seconds_total{mode="idle"}[5m])))
  legendFormat: CPU usage
  description: cluster cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-cpu-usage-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    level: cluster
    metric: cpu-usage-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: 1 - (avg(irate(node_cpu_seconds_total{mode="idle"}[5m])) by (instance))
  legendFormat: '[[instance]]'
  description: cluster cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-memory-usage-percent
  labels:
    app: metric-expression
    component: cluster
    details: "false"
    level: cluster
    metric: memory-usage-percent
    source: rancher-monitoring
spec:
  expression: 1 - sum(node_memory_MemAvailable_bytes) 
    / sum(node_memory_MemTotal_bytes) 
  legendFormat: Memory usage
  description: cluster memory usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cluster-memory-usage-percent-details
  labels:
    app: metric-expression
    component: cluster
    details: "true"
    level: cluster
    metric: memory-usage-percent
    source: rancher-monitoring
spec:
  expression: 1 - sum(node_memory_MemAvailable_bytes) by (instance)
    / sum(node_memory_MemTotal_bytes) by (instance)
  legendFormat: '[[instance]]'
  description: cluster memory usage percent
---

---
# Source: metric-expression-cluster/templates/expressioncontainer.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: container-cpu-cfs-throttled-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_cfs_throttled_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"}[5m]))
  legendFormat: CPU cfs throttled
  description: container cpu cfs throttled seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: cpu-usage-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"}[5m]))
  legendFormat: CPU usage
  description: container cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: container-cpu-system-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_system_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"}[5m]))
  legendFormat: CPU system seconds
  description: container cpu system seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: container-cpu-user-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_user_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"}[5m]))
  legendFormat: CPU user seconds
  description: container cpu user seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: container-memory-usage-bytes-sum-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    level: project
    metric: memory-usage-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_memory_working_set_bytes{name!~"POD", namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"})
  legendFormat: Memory usage
  description: container memory usage bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: container-disk-io-writes-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    graph: disk-io
    level: project
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_writes_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"}[5m]))
  legendFormat: Write
  description: container disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: container-disk-io-reads-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: container
    details: "true"
    graph: disk-io
    level: project
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_reads_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name=~"$containerName"}[5m]))
  legendFormat: Read
  description: container disk io reads bytes sum rate
---

---
# Source: metric-expression-cluster/templates/expressioncontrollermanager.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-volumes-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: volumes-depth
    source: rancher-monitoring
spec:
  expression: sum(volumes_depth) 
  legendFormat: Volumes depth
  description: controllermanager volumes depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-volumes-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: volumes-depth
    source: rancher-monitoring
spec:
  expression: sum(volumes_depth) by (instance)
  legendFormat: Volumes depth([[instance]])
  description: controllermanager volumes depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-deployment-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: deployment-depth
    source: rancher-monitoring
spec:
  expression: sum(deployment_depth) 
  legendFormat: Deployment depth
  description: controllermanager deployment adds
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-deployment-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: deployment-depth
    source: rancher-monitoring
spec:
  expression: sum(deployment_depth) by (instance)
  legendFormat: Deployment depth([[instance]])
  description: controllermanager deployment adds
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-replicaset-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: replicaset-depth
    source: rancher-monitoring
spec:
  expression: sum(replicaset_depth) 
  legendFormat: Replicaset depth
  description: controllermanager replicaset depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-replicaset-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: replicaset-depth
    source: rancher-monitoring
spec:
  expression: sum(replicaset_depth) by (instance)
  legendFormat: Replicaset depth([[instance]])
  description: controllermanager replicaset depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-service-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: service-depth
    source: rancher-monitoring
spec:
  expression: sum(service_depth) 
  legendFormat: Service depth
  description: controllermanager service depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-service-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: service-depth
    source: rancher-monitoring
spec:
  expression: sum(service_depth) by (instance)
  legendFormat: Service depth([[instance]])
  description: controllermanager service depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-serviceaccount-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: serviceaccount-depth
    source: rancher-monitoring
spec:
  expression: sum(serviceaccount_depth) 
  legendFormat: Serviceaccount depth
  description: controllermanager serviceaccount depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-serviceaccount-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: serviceaccount-depth
    source: rancher-monitoring
spec:
  expression: sum(serviceaccount_depth) by (instance)
  legendFormat: Serviceaccount depth([[instance]])
  description: controllermanager serviceaccount depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-endpoint-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: endpoint-depth
    source: rancher-monitoring
spec:
  expression: sum(endpoint_depth) 
  legendFormat: Endpoint depth
  description: controllermanager endpoint depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-endpoint-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: endpoint-depth
    source: rancher-monitoring
spec:
  expression: sum(endpoint_depth) by (instance)
  legendFormat: Endpoint depth([[instance]])
  description: controllermanager endpoint depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-daemonset-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: daemonset-depth
    source: rancher-monitoring
spec:
  expression: sum(daemonset_depth) 
  legendFormat: Daemonset depth
  description: controllermanager daemonset depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-daemonset-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: daemonset-depth
    source: rancher-monitoring
spec:
  expression: sum(daemonset_depth) by (instance)
  legendFormat: Daemonset depth([[instance]])
  description: controllermanager daemonset depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-deployment-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: deployment-depth
    source: rancher-monitoring
spec:
  expression: sum(deployment_depth) 
  legendFormat: Deployment depth
  description: controllermanager deployment depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-deployment-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: deployment-depth
    source: rancher-monitoring
spec:
  expression: sum(deployment_depth) by (instance)
  legendFormat: Deployment depth([[instance]])
  description: controllermanager deployment depth
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-statefulset-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: statefulset-depth
    source: rancher-monitoring
spec:
  expression: sum(statefulset_depth) 
  legendFormat: Statefulset depth
  description: controllermanager statefulset adds
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-statefulset-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: statefulset-depth
    source: rancher-monitoring
spec:
  expression: sum(statefulset_depth) by (instance)
  legendFormat: Statefulset depth([[instance]])
  description: controllermanager statefulset adds
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-replicationmanager-depth
  labels:
    app: metric-expression
    component: controllermanager
    details: "false"
    level: cluster
    metric: replicationmanager-depth
    source: rancher-monitoring
spec:
  expression: sum(replicationmanager_depth) 
  legendFormat: ReplicationManager depth
  description: controllermanager replicationmanager adds
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: controllermanager-replicationmanager-depth-details
  labels:
    app: metric-expression
    component: controllermanager
    details: "true"
    level: cluster
    metric: replicationmanager-depth
    source: rancher-monitoring
spec:
  expression: sum(replicationmanager_depth) by (instance)
  legendFormat: ReplicationManager depth([[instance]])
  description: controllermanager replicationmanager adds
---

---
# Source: metric-expression-cluster/templates/expressionetcd.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-failed-proposal
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    level: cluster
    metric: server-failed-proposal
    source: rancher-monitoring
spec:
  expression: sum(etcd_server_proposals_failed_total)
  legendFormat: Failed proposal
  description: etcd Server failed proposal
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-failed-proposal-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    level: cluster
    metric: server-failed-proposal
    source: rancher-monitoring
spec:
  expression: sum(etcd_server_proposals_failed_total)
  legendFormat: Failed proposal
  description: etcd Server failed proposal
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-leader-changes-seen-sum-increase
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    level: cluster
    metric: server-leader-changes-seen-sum-increase
    source: rancher-monitoring
spec:
  expression: max(etcd_server_leader_changes_seen_total)
  legendFormat: Number of leader changes per hour
  description: etcd server leader changes seen sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-leader-changes-seen-sum-increase-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    level: cluster
    metric: server-leader-changes-seen-sum-increase
    source: rancher-monitoring
spec:
  expression: max(etcd_server_leader_changes_seen_total)
  legendFormat: Number of leader changes per hour
  description: etcd server leader changes seen sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-client-receive-bytes-sum-rate
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: rpc-client-traffic
    level: cluster
    metric: grpc-client-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_client_grpc_received_bytes_total[5m]))
  legendFormat: Client traffic in
  description: etcd grpc client receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-client-receive-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: rpc-client-traffic
    level: cluster
    metric: grpc-client-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_client_grpc_received_bytes_total[5m])) by (instance)
  legendFormat: Client traffic in([[instance]])
  description: etcd grpc client receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-db-bytes-sum
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    level: cluster
    metric: db-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(etcd_debugging_mvcc_db_total_size_in_bytes) 
  legendFormat: DB size
  description: etcd db bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-db-bytes-sum-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    level: cluster
    metric: db-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(etcd_debugging_mvcc_db_total_size_in_bytes) by (instance)
  legendFormat: DB size([[instance]])
  description: etcd db bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-client-transmit-bytes-sum-rate
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: rpc-client-traffic
    level: cluster
    metric: grpc-client-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_client_grpc_sent_bytes_total[5m])) 
  legendFormat: Client traffic out
  description: etcd grpc client transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-client-transmit-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: rpc-client-traffic
    level: cluster
    metric: grpc-client-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_client_grpc_sent_bytes_total[5m])) by (instance)
  legendFormat: Client traffic out([[instance]])
  description: etcd grpc client transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-leader-sum
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    level: cluster
    metric: server-leader-sum
    source: rancher-monitoring
spec:
  expression: max(etcd_server_has_leader)
  legendFormat: Has leader
  description: etcd server leader sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-leader-sum-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    level: cluster
    metric: server-leader-sum
    source: rancher-monitoring
spec:
  expression: max(etcd_server_has_leader)
  legendFormat: Has leader
  description: etcd server leader sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-committed-sum-increase
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: proposal
    level: cluster
    metric: server-proposals-committed-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_server_proposals_committed_total[5m])) 
  legendFormat: Proposal commit rate
  description: etcd server proposals committed sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-committed-sum-increase-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: proposal
    level: cluster
    metric: server-proposals-committed-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_server_proposals_committed_total[5m])) by (instance)
  legendFormat: Proposal commit rate([[instance]])
  description: etcd server proposals committed sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-applied-sum-increase
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: proposal
    level: cluster
    metric: server-proposals-applied-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_server_proposals_applied_total[5m])) 
  legendFormat: Proposal applied
  description: etcd server proposals applied sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-applied-sum-increase-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: proposal
    level: cluster
    metric: server-proposals-applied-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_server_proposals_applied_total[5m])) by (instance)
  legendFormat: Proposal applied([[instance]])
  description: etcd server proposals applied sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-failed-sum-increase
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: proposal
    level: cluster
    metric: server-proposals-failed-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_server_proposals_failed_total[5m])) 
  legendFormat: Proposal failed
  description: etcd server proposals failed sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-failed-sum-increase-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: proposal
    level: cluster
    metric: server-proposals-failed-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_server_proposals_failed_total[5m])) by (instance)
  legendFormat: Proposal failed([[instance]])
  description: etcd server proposals failed sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-pending-sum-increase
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: proposal
    level: cluster
    metric: server-proposals-pending-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(etcd_server_proposals_pending) 
  legendFormat: Proposal pending
  description: etcd server proposals pending sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-server-proposals-pending-sum-increase-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: proposal
    level: cluster
    metric: server-proposals-pending-sum-increase
    source: rancher-monitoring
spec:
  expression: sum(etcd_server_proposals_pending) by (instance)
  legendFormat: Proposal pending([[instance]])
  description: etcd server proposals pending sum increase
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-disk-wal-fsync-duration-seconds-sum-quantile
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: sync-duration
    level: cluster
    metric: disk-wal-fsync-duration-seconds-sum-quantile
    source: rancher-monitoring
spec:
  expression: sum(histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])) by (instance, le)))
  legendFormat: WAL fsync
  description: etcd disk wal fsync duration seconds sum quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-disk-wal-fsync-duration-seconds-sum-quantile-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: sync-duration
    level: cluster
    metric: disk-wal-fsync-duration-seconds-sum-quantile
    source: rancher-monitoring
spec:
  expression: histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])) by (instance, le))
  legendFormat: WAL fsync([[instance]])
  description: etcd disk wal fsync duration seconds sum quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-request-error-percent
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    level: cluster
    metric: grpc-request-error-percent
    source: rancher-monitoring
spec:
  expression: sum(rate(grpc_server_handled_total{grpc_code!="OK"}[5m]))  / sum(rate(grpc_server_handled_total[5m]))
  legendFormat: Rpc failed rate
  description: etcd grpc request error percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-request-error-percent-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    level: cluster
    metric: grpc-request-error-percent
    source: rancher-monitoring
spec:
  expression: sum(rate(grpc_server_handled_total{grpc_code!="OK"}[5m])) by (instance)
    / sum(rate(grpc_server_handled_total[5m])) by (instance)
  legendFormat: RPC failed rate([[instance]])
  description: etcd grpc request error percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-disk-commit-duration-seconds-sum-quantile
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: sync-duration
    level: cluster
    metric: disk-commit-duration-seconds-sum-quantile
    source: rancher-monitoring
spec:
  expression: sum(histogram_quantile(0.99, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket[5m])) by (instance, le)))
  legendFormat: DB fsync
  description: etcd disk commit duration seconds sum quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-disk-commit-duration-seconds-sum-quantile-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: sync-duration
    level: cluster
    metric: disk-commit-duration-seconds-sum-quantile
    source: rancher-monitoring
spec:
  expression: histogram_quantile(0.99, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket[5m])) by (instance, le))
  legendFormat: DB fsync([[instance]])
  description: etcd disk commit duration seconds sum quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-request-slow-quantile
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    level: cluster
    metric: grpc-request-slow-quantile
    source: rancher-monitoring
spec:
  expression: sum(histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket{grpc_service=~"etcdserverpb.*",grpc_type="unary"}[5m])) by (instance,le)))
  legendFormat: Request slow"
  description: etcd grpc request slow quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-grpc-request-slow-quantile-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    level: cluster
    metric: grpc-request-slow-quantile
    source: rancher-monitoring
spec:
  expression: histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket{grpc_service=~"etcdserverpb.*",grpc_type="unary"}[5m])) by (instance,le))
  legendFormat: Request slow([[instance]])
  description: etcd grpc request slow quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-active-watch-stream
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: etcd-stream
    level: cluster
    metric: active-watch-stream
    source: rancher-monitoring
spec:
  expression: sum(grpc_server_started_total{grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"})
     - sum(grpc_server_handled_total{grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"})
  legendFormat: Watch streams
  description: Etcd watch stream
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-active-watch-stream-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: etcd-stream
    level: cluster
    metric: active-watch-stream
    source: rancher-monitoring
spec:
  expression: sum(grpc_server_started_total{grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"})
    by (instance) - sum(grpc_server_handled_total{grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"})
    by (instance)
  legendFormat: Watch streams([[instance]])
  description: Etcd watch stream
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-lease-watch-stream
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: etcd-stream
    level: cluster
    metric: lease-watch-stream
    source: rancher-monitoring
spec:
  expression: sum(grpc_server_started_total{grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"})
     - sum(grpc_server_handled_total{grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"})
  legendFormat: Lease watch stream
  description: Etcd lease stream
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-lease-watch-stream-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: etcd-stream
    level: cluster
    metric: lease-watch-stream
    source: rancher-monitoring
spec:
  expression: sum(grpc_server_started_total{grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"})
    by (instance) - sum(grpc_server_handled_total{grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"})
    by (instance)
  legendFormat: Lease watch stream([[instance]])
  description: Etcd lease stream
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-peer-traffic-in
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: etcd-peer-traffic
    level: cluster
    metric: peer-traffic-in
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_peer_received_bytes_total[5m]))
  legendFormat: Traffic in"
  description: Etcd peer traffic in
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-peer-traffic-in-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: etcd-peer-traffic
    level: cluster
    metric: peer-traffic-in
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_peer_received_bytes_total[5m])) by (instance)
  legendFormat: Traffic in([[instance]])
  description: Etcd peer traffic in
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-peer-traffic-out
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: etcd-peer-traffic
    level: cluster
    metric: peer-traffic-out
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_peer_sent_bytes_total[5m])) 
  legendFormat: Traffic out"
  description: Etcd peer traffic out
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-peer-traffic-out-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: etcd-peer-traffic
    level: cluster
    metric: peer-traffic-out
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_network_peer_sent_bytes_total[5m])) by (instance)
  legendFormat: Traffic out([[instance]])
  description: Etcd peer traffic out
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-rpc-rate
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: rpc-rate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(grpc_server_started_total{grpc_type="unary"}[5m])) 
  legendFormat: RPC rate
  description: rpc-rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-rpc-rate-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: rpc-rate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(grpc_server_started_total{grpc_type="unary"}[5m])) by (instance)
  legendFormat: Rpc rate([[instance]])
  description: rpc-rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-rpc-rate-failed
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: rpc-rate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(grpc_server_handled_total{grpc_type="unary",grpc_code!="OK"}[5m]))
  legendFormat: Rpc failed rate
  description: rpc-rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-rpc-rate-failed-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: rpc-rate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(grpc_server_handled_total{grpc_type="unary",grpc_code!="OK"}[5m])) by (instance)
  legendFormat: Rpc failed rate([[instance]])
  description: rpc-rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-latency-distributions-of-commit-called-by-backend
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: disk-operate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_disk_backend_commit_duration_seconds_sum[1m])) 
  legendFormat: Commit latency called by backend
  description: The latency distributions of commit called by backend
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-latency-distributions-of-commit-called-by-backend-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: disk-operate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_disk_backend_commit_duration_seconds_sum[1m])) by (instance)
  legendFormat: Commit latency called by backend([[instance]])
  description: The latency distributions of commit called by backend
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-latency-distributions-of-fsync-called-by-wal
  labels:
    app: metric-expression
    component: etcd
    details: "false"
    graph: disk-operate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_disk_wal_fsync_duration_seconds_sum[1m])) 
  legendFormat: Fsync latency called by wal
  description: The latency distributions of fsync called by wal
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: etcd-latency-distributions-of-fsync-called-by-wal-details
  labels:
    app: metric-expression
    component: etcd
    details: "true"
    graph: disk-operate
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(rate(etcd_disk_wal_fsync_duration_seconds_sum[1m])) by (instance)
  legendFormat: Fsync latency called by wal([[instance]])
  description: The latency distributions of fsync called by wal
---

---
# Source: metric-expression-cluster/templates/expressionfluentd.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: input-record-number
  labels:
    app: metric-expression
    component: fluentd
    details: "false"
    level: cluster
    metric: input-record
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_input_status_num_records_total[5m])) 
  legendFormat: Input record number
  description: Fluentd input status num records total
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: input-record-number-details
  labels:
    app: metric-expression
    component: fluentd
    details: "true"
    level: cluster
    metric: input-record
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_input_status_num_records_total[5m])) by (instance)
  legendFormat: Input record number([[instance]])
  description: Fluentd input status num records total
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: output-record-number
  labels:
    app: metric-expression
    component: fluentd
    details: "false"
    level: cluster
    metric: output-record
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_output_status_num_records_total[5m])) 
  legendFormat: Output record number
  description: Fluentd output status num records total
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: output-record-number-details
  labels:
    app: metric-expression
    component: fluentd
    details: "true"
    level: cluster
    metric: output-record
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_output_status_num_records_total[5m])) by (instance)
  legendFormat: Output record number([[instance]])
  description: Fluentd output status num records total
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: output-errors
  labels:
    app: metric-expression
    component: fluentd
    details: "false"
    level: cluster
    metric: output-errors
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_output_status_num_errors[5m])) 
  legendFormat: Plugin Output errors
  description: Fluentd output errors number
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: output-errors-details
  labels:
    app: metric-expression
    component: fluentd
    details: "true"
    level: cluster
    metric: output-errors
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_output_status_num_errors[5m])) by (type)
  legendFormat: Plugin([[type]])
  description: Fluentd output errors number
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: buffer-queue-length
  labels:
    app: metric-expression
    component: fluentd
    details: "false"
    level: cluster
    metric: buffer-queue-length
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_output_status_buffer_queue_length[5m])) 
  legendFormat: Buffer queue
  description: Fluentd Buffer queue length
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: buffer-queue-length-details
  labels:
    app: metric-expression
    component: fluentd
    details: "true"
    level: cluster
    metric: buffer-queue-length
    source: rancher-monitoring
spec:
  expression: sum(rate(fluentd_output_status_buffer_queue_length[5m])) by (instance)
  legendFormat: '[[instance]]'
  description: Fluentd Buffer queue length
---

---
# Source: metric-expression-cluster/templates/expressioningresscontroller.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-reading
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(nginx_ingress_controller_nginx_process_connections{state="reading"})
  legendFormat: Reading
  description: ingresscontroller nginx connection reading
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-reading-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(nginx_ingress_controller_nginx_process_connections{state="reading"}) by (instance)
  legendFormat: Reading([[instance]])
  description: ingresscontroller nginx connection reading
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-waiting
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(nginx_ingress_controller_nginx_process_connections{state="waiting"})
  legendFormat: Waiting
  description: ingresscontroller nginx connection waiting
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-waiting-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(nginx_ingress_controller_nginx_process_connections{state="waiting"}) by (instance)
  legendFormat: Waiting([[instance]])
  description: ingresscontroller nginx connection waiting
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-writing
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(nginx_ingress_controller_nginx_process_connections{state="writing"})
  legendFormat: Writing
  description: ingresscontroller nginx connection writing
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-writing-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(nginx_ingress_controller_nginx_process_connections{state="writing"}) by (instance)
  legendFormat: Writing([[instance]])
  description: ingresscontroller nginx connection writing
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-accepted
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(ceil(increase(nginx_ingress_controller_nginx_process_connections_total{state="accepted"}[5m])))
  legendFormat: Accepted
  description: ingresscontroller nginx connection accepted
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-accepted-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(ceil(increase(nginx_ingress_controller_nginx_process_connections_total{state="accepted"}[5m]))) by (instance)
  legendFormat: Accepted([[instance]])
  description: ingresscontroller nginx connection accepted
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-active
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(ceil(increase(nginx_ingress_controller_nginx_process_connections_total{state="active"}[5m])))
  legendFormat: Active
  description: ingresscontroller nginx connection active
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-active-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(ceil(increase(nginx_ingress_controller_nginx_process_connections_total{state="active"}[5m]))) by (instance)
  legendFormat: Active([[instance]])
  description: ingresscontroller nginx connection active
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-handled
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(ceil(increase(nginx_ingress_controller_nginx_process_connections_total{state="handled"}[5m])))
  legendFormat: Handled
  description: ingresscontroller nginx connection handled
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-connection-handled-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    graph: nginx-connection
    level: cluster
    source: rancher-monitoring
spec:
  expression: sum(ceil(increase(nginx_ingress_controller_nginx_process_connections_total{state="handled"}[5m]))) by (instance)
  legendFormat: Handled([[instance]])
  description: ingresscontroller nginx connection handled
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-process-seconds-by-host
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "false"
    level: cluster
    metric: request-process-seconds
    source: rancher-monitoring
spec:
  expression: topk(10, histogram_quantile(0.95,sum by (le, host)(rate(nginx_ingress_controller_request_duration_seconds_bucket{host!="_"}[5m]))))
  legendFormat: Request duration(host:[[host]])
  description: top 10 ingresscontroller nginx request duration by host
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: ingresscontroller-nginx-process-seconds-by-host-details
  labels:
    app: metric-expression
    component: ingresscontroller
    details: "true"
    level: cluster
    metric: request-process-seconds
    source: rancher-monitoring
spec:
  expression:  topk(10, histogram_quantile(0.95,sum by (le, host, path)(rate(nginx_ingress_controller_request_duration_seconds_bucket{host!="_"}[5m]))))
  legendFormat: Request duration(host:[[host]] path:[[path]])
  description: top 10 ingresscontroller nginx request duration by path
---

---
# Source: metric-expression-cluster/templates/expressionnode.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-bytes-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-io
    level: cluster
    metric: network-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Transmit
  description: node network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-io
    level: cluster
    metric: network-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: 'Transmit([[device]])'
  description: node network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-packets-dropped-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Receive packets
  description: node network receive packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-packets-dropped-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Receive packets([[device]])
  description: node network receive packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-packets-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Transmit packets
  description: node network transmit packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-packets-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Transmit packets([[device]])
  description: node network transmit packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-disk-io-writes-bytes-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: disk-io
    level: cluster
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_written_bytes_total{instance=~"$instance", device!~"HarddiskVolume.+"}[5m]))
  legendFormat: Write
  description: node disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-disk-io-writes-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: disk-io
    level: cluster
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_written_bytes_total{instance=~"$instance", device!~"HarddiskVolume.+"}[5m]))
    by (device)
  legendFormat: Write([[device]])
  description: node disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-disk-io-reads-bytes-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: disk-io
    level: cluster
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_read_bytes_total{instance=~"$instance", device!~"HarddiskVolume.+"}[5m]))
  legendFormat: Read
  description: node disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-disk-io-reads-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: disk-io
    level: cluster
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_disk_read_bytes_total{instance=~"$instance", device!~"HarddiskVolume.+"}[5m])) by
    (device)
  legendFormat: Read([[device]])
  description: node disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-fs-usage-percent
  labels:
    app: metric-expression
    component: node
    details: "false"
    level: cluster
    metric: fs-usage-percent
    source: rancher-monitoring
spec:
  expression: (sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+",instance=~"$instance"})
     - sum(node_filesystem_free_bytes{device!~"rootfs|HarddiskVolume.+",instance=~"$instance"})
    ) / sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+",instance=~"$instance"})
  legendFormat: Disk usage
  description: node fs usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-fs-usage-percent-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    level: cluster
    metric: fs-usage-percent
    source: rancher-monitoring
spec:
  expression: (sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+",instance=~"$instance"})
    by (device) - sum(node_filesystem_free_bytes{device!~"rootfs|HarddiskVolume.+",instance=~"$instance"})
    by (device)) / sum(node_filesystem_size_bytes{device!~"rootfs|HarddiskVolume.+",instance=~"$instance"})
    by (device)
  legendFormat: '[[device]]'
  description: node fs usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-packets-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Receive packets
  description: node network receive packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-packets-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-receive-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_packets_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Receive packets([[device]])
  description: node network receive packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-errors-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-transmit-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Transmit errors
  description: node network transmit errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-errors-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-transmit-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Transmit errors([[device]])
  description: node network transmit errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-load-1
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: cpu-load
    level: cluster
    metric: cpu-load-1
    source: rancher-monitoring
spec:
  expression: sum(node_load1{instance=~"$instance"})
  legendFormat: Load1
  description: node cpu load 1
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-load-1-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: cpu-load
    level: cluster
    metric: cpu-load-1
    source: rancher-monitoring
spec:
  expression: sum(node_load1{instance=~"$instance"})
  legendFormat: Load1
  description: node cpu load 1
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-load-15
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: cpu-load
    level: cluster
    metric: cpu-load-15
    source: rancher-monitoring
spec:
  expression: sum(node_load15{instance=~"$instance"})
  legendFormat: Load15
  description: node cpu load 15
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-load-15-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: cpu-load
    level: cluster
    metric: cpu-load-15
    source: rancher-monitoring
spec:
  expression: sum(node_load15{instance=~"$instance"})
  legendFormat: Load15
  description: node cpu load 15
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-usage-seconds-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    level: cluster
    metric: cpu-usage-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: 1 - (avg(irate(node_cpu_seconds_total{mode="idle", instance=~"$instance"}[5m])) by (instance))
  legendFormat: CPU
  description: node cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-usage-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    level: cluster
    metric: cpu-usage-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: avg(irate(node_cpu_seconds_total{mode!="idle", instance=~"$instance"}[5m]))by (mode) 
  legendFormat: '[[mode]]'
  description: node cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-memory-usage-percent
  labels:
    app: metric-expression
    component: node
    details: "false"
    level: cluster
    metric: memory-usage-percent
    source: rancher-monitoring
spec:
  expression: 1 - sum(node_memory_MemAvailable_bytes{instance=~"$instance"}) 
    / sum(node_memory_MemTotal_bytes{instance=~"$instance"}) 
  legendFormat: Memory usage
  description: node memory usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-memory-usage-percent-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    level: cluster
    metric: memory-usage-percent
    source: rancher-monitoring
spec:
  expression: 1 - sum(node_memory_MemAvailable_bytes{instance=~"$instance"}) 
    / sum(node_memory_MemTotal_bytes{instance=~"$instance"}) 
  legendFormat: Memory usage
  description: node memory usage percent
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-bytes-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-io
    level: cluster
    metric: network-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Receive
  description: node network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-io
    level: cluster
    metric: network-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Receive([[device]])
  description: node network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-errors-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-receive-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Receive packets
  description: node network receive errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-receive-errors-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-receive-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_receive_errs_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Receive packets([[device]])
  description: node network receive errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-load-5
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: cpu-load
    level: cluster
    metric: cpu-load-5
    source: rancher-monitoring
spec:
  expression: sum(node_load5{instance=~"$instance"})
  legendFormat: Load5
  description: node cpu load 5
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-cpu-load-5-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: cpu-load
    level: cluster
    metric: cpu-load-5
    source: rancher-monitoring
spec:
  expression: sum(node_load5{instance=~"$instance"})
  legendFormat: Load5
  description: node cpu load 5
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-packets-dropped-sum-rate
  labels:
    app: metric-expression
    component: node
    details: "false"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
  legendFormat: Transmit dropped
  description: node network transmit packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: node-network-transmit-packets-dropped-sum-rate-details
  labels:
    app: metric-expression
    component: node
    details: "true"
    graph: network-packet
    level: cluster
    metric: network-transmit-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(node_network_transmit_drop_total{device!~"lo|veth.*|docker.*|flannel.*|cali.*|cbr.*",instance=~"$instance"}[5m]))
    by (device)
  legendFormat: Transmit dropped([[device]])
  description: node network transmit packets dropped sum rate
---

---
# Source: metric-expression-cluster/templates/expressionpod.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-cfs-throttled-seconds-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_cfs_throttled_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU cfs throttled
  description: pod cpu cfs throttled seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-cfs-throttled-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_cfs_throttled_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (container_name)
  legendFormat: CPU cfs throttled([[container_name]])
  description: pod cpu cfs throttled seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-fs-bytes-sum
  labels:
    app: metric-expression
    component: pod
    details: "false"
    level: project
    metric: fs-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_fs_usage_bytes{namespace=~"$namespace", pod_name=~"$podName",
    container_name!=""}) 
  legendFormat: Filesystem usage
  description: pod fs bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-fs-bytes-sum-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    level: project
    metric: fs-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_fs_usage_bytes{namespace=~"$namespace", pod_name=~"$podName",
    container_name!=""}) by (container_name)
  legendFormat: Filesystem usage([[container_name]])
  description: pod fs bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-packets-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-packet
    level: project
    metric: network-receive-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Receive packets
  description: pod network receive packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-packets-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-packet
    level: project
    metric: network-receive-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Receive packets
  description: pod network receive packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-packets-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-packet
    level: project
    metric: network-transmit-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Transmit packets
  description: pod network transmit packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-packets-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-packet
    level: project
    metric: network-transmit-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Transmit packets
  description: pod network transmit packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-user-seconds-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_user_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU user seconds
  description: pod cpu user seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-user-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_user_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (container_name)
  legendFormat: CPU user seconds([[container_name]])
  description: pod cpu user seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-disk-io-reads-bytes-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: disk-io
    level: project
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_reads_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Read
  description: pod disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-disk-io-reads-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: disk-io
    level: project
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_reads_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (container_name)
  legendFormat: Read([[container_name]])
  description: pod disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-bytes-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-io
    level: project
    metric: network-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Receive
  description: pod network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-io
    level: project
    metric: network-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Receive
  description: pod network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-bytes-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-io
    level: project
    metric: network-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Transmit
  description: pod network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-io
    level: project
    metric: network-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Transmit
  description: pod network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-packets-dropped-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-packet
    level: project
    metric: network-receive-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Receive dropped
  description: pod network receive packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-packets-dropped-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-packet
    level: project
    metric: network-receive-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Receive dropped
  description: pod network receive packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-memory-usage-bytes-sum
  labels:
    app: metric-expression
    component: pod
    details: "false"
    level: project
    metric: memory-usage-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_memory_working_set_bytes{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}) 
  legendFormat: Memory usage
  description: pod memory usage bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-memory-usage-bytes-sum-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    level: project
    metric: memory-usage-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_memory_working_set_bytes{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}) by (container_name)
  legendFormat: Memory usage([[container_name]])
  description: pod memory usage bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-disk-io-writes-bytes-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: disk-io
    level: project
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_writes_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Write
  description: pod disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-disk-io-writes-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: disk-io
    level: project
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_writes_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (container_name)
  legendFormat: Write([[container_name]])
  description: pod disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-errors-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-packet
    level: project
    metric: network-receive-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Receive errors
  description: pod network receive errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-receive-errors-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-packet
    level: project
    metric: network-receive-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Receive errors
  description: pod network receive errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-usage-seconds-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_usage_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU usage
  description: pod CPU usage sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-usage-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_usage_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (container_name)
  legendFormat: CPU usage([[container_name]])
  description: pod CPU usage sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-errors-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-packet
    level: project
    metric: network-transmit-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Transmit errors
  description: pod network transmit errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-errors-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-packet
    level: project
    metric: network-transmit-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Transmit errors
  description: pod network transmit errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-packets-dropped-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: network-packet
    level: project
    metric: network-transmit-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Transmit dropped
  description: pod network transmit packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-network-transmit-packets-dropped-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: network-packet
    level: project
    metric: network-transmit-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Transmit dropped
  description: pod network transmit packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-system-seconds-sum-rate
  labels:
    app: metric-expression
    component: pod
    details: "false"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_system_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU system seconds
  description: pod cpu system seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: pod-cpu-system-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: pod
    details: "true"
    graph: cpu-usage
    level: project
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_system_seconds_total{container_name!="POD",namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (container_name)
  legendFormat: CPU system seconds([[container_name]])
  description: pod cpu system seconds sum rate
---

---
# Source: metric-expression-cluster/templates/expressionscheduler.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: scheduler-e-2-e-scheduling-latency-seconds-quantile
  labels:
    app: metric-expression
    component: scheduler
    details: "false"
    level: cluster
    metric: e-2-e-scheduling-latency-seconds-quantile
    source: rancher-monitoring
spec:
  expression: sum(histogram_quantile(0.99, sum(scheduler_e2e_scheduling_latency_microseconds_bucket) by (le, instance)) / 1e+06)
  legendFormat: E2E latency
  description: scheduler e 2 e scheduling latency seconds quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: scheduler-e-2-e-scheduling-latency-seconds-quantile-details
  labels:
    app: metric-expression
    component: scheduler
    details: "true"
    level: cluster
    metric: e-2-e-scheduling-latency-seconds-quantile
    source: rancher-monitoring
spec:
  expression: histogram_quantile(0.99, sum(scheduler_e2e_scheduling_latency_microseconds_bucket) by (le, instance)) / 1e+06
  legendFormat: E2E latency([[instance]])
  description: scheduler e 2 e scheduling latency seconds quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: scheduler-total-preemption-attempts
  labels:
    app: metric-expression
    component: scheduler
    details: "false"
    level: cluster
    metric: total-preemption-attempts
    source: rancher-monitoring
spec:
  expression: sum(rate(scheduler_total_preemption_attempts[5m])) by (instance)
  legendFormat: Preemption attempts
  description: Scheduler scheduling algorithm latency seconds quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: scheduler-total-preemption-attempts-details
  labels:
    app: metric-expression
    component: scheduler
    details: "true"
    level: cluster
    metric: total-preemption-attempts
    source: rancher-monitoring
spec:
  expression: sum(rate(scheduler_total_preemption_attempts[5m])) by (instance)
  legendFormat: Preemption attempts([[instance]])
  description: Scheduler scheduling algorithm latency seconds quantile
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: scheduler-pod-unscheduler
  labels:
    app: metric-expression
    component: scheduler
    details: "false"
    level: cluster
    metric: pod-unscheduler
    source: rancher-monitoring
spec:
  expression: sum(kube_pod_status_scheduled{condition="false"})
  legendFormat: Scheduling failed pods
  description: pod unscheduler
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: scheduler-pod-unscheduler-details
  labels:
    app: metric-expression
    component: scheduler
    details: "true"
    level: cluster
    metric: pod-unscheduler
    source: rancher-monitoring
spec:
  expression: sum(kube_pod_status_scheduled{condition="false"})
  legendFormat: Scheduling failed pods
  description: pod unscheduler
---

---
# Source: metric-expression-cluster/templates/expressionworkload.yaml
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-disk-io-writes-bytes-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: disk-io
    level: project
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_writes_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Write
  description: workload disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-disk-io-writes-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: disk-io
    level: project
    metric: disk-io-writes-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_writes_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Write([[pod_name]])
  description: workload disk io writes bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-disk-io-reads-bytes-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: disk-io
    level: project
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_reads_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Read
  description: workload disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-disk-io-reads-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: disk-io
    level: project
    metric: disk-io-reads-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_fs_reads_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Read([[pod_name]])
  description: workload disk io reads bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-fs-bytes-sum
  labels:
    app: metric-expression
    component: workload
    details: "false"
    level: project
    metric: fs-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_fs_usage_bytes{namespace=~"$namespace", pod_name=~"$podName",
    container_name!=""}) 
  legendFormat: File usage
  description: workload fs bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-fs-bytes-sum-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    level: project
    metric: fs-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_fs_usage_bytes{namespace=~"$namespace", pod_name=~"$podName",
    container_name!=""}) by (pod_name)
  legendFormat: pod_name]]
  description: workload fs bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-packets-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-packet
    level: project
    metric: network-transmit-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Transmit packets
  description: workload network transmit packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-packets-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-packet
    level: project
    metric: network-transmit-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Transmit packets([[pod_name]])
  description: workload network transmit packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-packets-dropped-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-packet
    level: project
    metric: network-receive-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Receive dropped
  description: workload network receive packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-packets-dropped-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-packet
    level: project
    metric: network-receive-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Receive dropped([[pod_name]])
  description: workload network receive packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-usage-seconds-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: cpu-usage
    level: project
    metric: cpu-usage-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU usage
  description: workload cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-usage-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: cpu-usage
    level: project
    metric: cpu-usage-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: CPU usage([[pod_name]])
  description: workload cpu usage seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-system-seconds-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: cpu-usage
    level: project
    metric: cpu-system-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_system_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU system seconds
  description: workload cpu system seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-system-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: cpu-usage
    level: project
    metric: cpu-system-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_system_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: CPU system seconds([[pod_name]])
  description: workload cpu system seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-bytes-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-io
    level: project
    metric: network-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Receive
  description: workload network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-io
    level: project
    metric: network-receive-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Receive([[pod_name]])
  description: workload network receive bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-errors-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-packet
    level: project
    metric: network-receive-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Receive errors
  description: workload network receive errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-errors-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-packet
    level: project
    metric: network-receive-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Receive errors([[pod_name]])
  description: workload network receive errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-packets-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-packet
    level: project
    metric: network-receive-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Receive packets
  description: workload network receive packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-receive-packets-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-packet
    level: project
    metric: network-receive-packets-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_receive_packets_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Receive packets([[pod_name]])
  description: workload network receive packets sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-memory-usage-bytes-sum
  labels:
    app: metric-expression
    component: workload
    details: "false"
    level: project
    metric: memory-usage-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_memory_working_set_bytes{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}) 
  legendFormat: Memory
  description: workload memory usage bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-memory-usage-bytes-sum-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    level: project
    metric: memory-usage-bytes-sum
    source: rancher-monitoring
spec:
  expression: sum(container_memory_working_set_bytes{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}) by (pod_name)
  legendFormat: '[[pod_name]]'
  description: workload memory usage bytes sum
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-bytes-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-io
    level: project
    metric: network-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m]))
  legendFormat: Transmit
  description: workload network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-bytes-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-io
    level: project
    metric: network-transmit-bytes-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_bytes_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Transmit([[pod_name]])
  description: workload network transmit bytes sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-errors-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-packet
    level: project
    metric: network-transmit-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Transmit errors
  description: workload network transmit errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-errors-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-packet
    level: project
    metric: network-transmit-errors-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_errors_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Transmit errors([[pod_name]])
  description: workload network transmit errors sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-packets-dropped-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: network-packet
    level: project
    metric: network-transmit-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: Transmit dropped
  description: workload network transmit packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-network-transmit-packets-dropped-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: network-packet
    level: project
    metric: network-transmit-packets-dropped-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_network_transmit_packets_dropped_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: Transmit dropped([[pod_name]])
  description: workload network transmit packets dropped sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-user-seconds-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: cpu-usage
    level: project
    metric: cpu-user-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_user_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU user seconds
  description: workload cpu user seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-user-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: cpu-usage
    level: project
    metric: cpu-user-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_user_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: CPU user seconds([[pod_name]])
  description: workload cpu user seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-cfs-throttled-seconds-sum-rate
  labels:
    app: metric-expression
    component: workload
    details: "false"
    graph: cpu-usage
    level: project
    metric: cpu-cfs-throttled-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_cfs_throttled_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) 
  legendFormat: CPU cfs throttled
  description: workload cpu cfs throttled seconds sum rate
---
kind: MonitorMetric
apiVersion: management.cattle.io/v3
metadata:
  name: workload-cpu-cfs-throttled-seconds-sum-rate-details
  labels:
    app: metric-expression
    component: workload
    details: "true"
    graph: cpu-usage
    level: project
    metric: cpu-cfs-throttled-seconds-sum-rate
    source: rancher-monitoring
spec:
  expression: sum(rate(container_cpu_cfs_throttled_seconds_total{namespace=~"$namespace",pod_name=~"$podName",
    container_name!=""}[5m])) by (pod_name)
  legendFormat: CPU cfs throttled([[pod_name]])
  description: workload cpu cfs throttled seconds sum rate
---
`

	ProjectMetricExpression = `
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: container
  name: container-cpu-usage
spec:
  resourceType: container
  priority: 800
  title: container-cpu-usage
  detailsMetricsSelector:
    details: "true"
    component: container
    graph: cpu-usage
  yAxis:
    unit: mcpu
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: container
  name: container-memory-usage-bytes-sum
spec:
  resourceType: container
  priority: 801
  title: container-memory-usage-bytes-sum
  detailsMetricsSelector:
    details: "true"
    component: container
    metric: memory-usage-bytes-sum
  yAxis:
    unit: byte
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: container
  name: container-disk-io
spec:
  resourceType: container
  priority: 804
  title: container-disk-io
  detailsMetricsSelector:
    details: "true"
    component: container
    graph: disk-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: pod
  name: pod-cpu-usage
spec:
  resourceType: pod
  priority: 700
  title: pod-cpu-usage
  metricsSelector:
    details: "false"
    component: pod
    graph: cpu-usage
  detailsMetricsSelector:
    details: "true"
    component: pod
    graph: cpu-usage
  yAxis:
    unit: mcpu
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: pod
  name: pod-memory-usage-bytes-sum
spec:
  resourceType: pod
  priority: 701
  title: pod-memory-usage-bytes-sum
  metricsSelector:
    details: "false"
    component: pod
    metric: memory-usage-bytes-sum
  detailsMetricsSelector:
    details: "true"
    component: pod
    metric: memory-usage-bytes-sum
  yAxis:
    unit: byte
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: pod
  name: pod-network-io
spec:
  resourceType: pod
  priority: 702
  title: pod-network-io
  metricsSelector:
    details: "false"
    component: pod
    graph: network-io
  detailsMetricsSelector:
    details: "true"
    component: pod
    graph: network-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: pod
  name: pod-network-packet
spec:
  resourceType: pod
  priority: 703
  title: pod-network-packet
  metricsSelector:
    details: "false"
    component: pod
    graph: network-packet
  detailsMetricsSelector:
    details: "true"
    component: pod
    graph: network-packet
  yAxis:
    unit: pps
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: pod
  name: pod-disk-io
spec:
  resourceType: pod
  priority: 704
  title: pod-disk-io
  metricsSelector:
    details: "false"
    component: pod
    graph: disk-io
  detailsMetricsSelector:
    details: "true"
    component: pod
    graph: disk-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: workload
  name: workload-cpu-usage
spec:
  resourceType: workload
  priority: 600
  title: workload-cpu-usage
  metricsSelector:
    details: "false"
    component: workload
    graph: cpu-usage
  detailsMetricsSelector:
    details: "true"
    component: workload
    graph: cpu-usage
  yAxis:
    unit: mcpu
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: workload
  name: workload-memory-usage-bytes-sum
spec:
  resourceType: workload
  priority: 601
  title: workload-memory-usage-bytes-sum
  metricsSelector:
    details: "false"
    component: workload
    metric: memory-usage-bytes-sum
  detailsMetricsSelector:
    details: "true"
    component: workload
    metric: memory-usage-bytes-sum
  yAxis:
    unit: byte
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: workload
  name: workload-network-io
spec:
  resourceType: workload
  priority: 602
  title: workload-network-io
  metricsSelector:
    details: "false"
    component: workload
    graph: network-io
  detailsMetricsSelector:
    details: "true"
    component: workload
    graph: network-io
  yAxis:
    unit: bps
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: workload
  name: workload-network-packet
spec:
  resourceType: workload
  priority: 603
  title: workload-network-packet
  metricsSelector:
    details: "false"
    component: workload
    graph: network-packet
  detailsMetricsSelector:
    details: "true"
    component: workload
    graph: network-packet
  yAxis:
    unit: pps
---
apiVersion: management.cattle.io/v3
kind: ProjectMonitorGraph
metadata:
  labels:
    app: metric-expression
    source: rancher-monitoring
    level: project
    component: workload
  name: workload-disk-io
spec:
  resourceType: workload
  priority: 604
  title: workload-disk-io
  metricsSelector:
    details: "false"
    component: workload
    graph: disk-io
  detailsMetricsSelector:
    details: "true"
    component: workload
    graph: disk-io
  yAxis:
    unit: bps
---
`
)
