package monitoring

import (
	"fmt"
	"github.com/rancher/norman/types"
)

type AppLevel string

const (
	cattleNamespaceName = "cattle-prometheus"
)

const (
	// The names of App
	projectLevelAppName             = "project-monitoring"
	clusterLevelAlertManagerAppName = "cluster-alerting"

	// The headless service name of Prometheus
	alertManagerHeadlessServiceName = "alertmanager-operated"

	//CattlePrometheusRuleLabelKey The label info of PrometheusRule
	CattlePrometheusRuleLabelKey           = "source"
	CattleAlertingPrometheusRuleLabelValue = "rancher-alert"
	RancherMonitoringTemplateName          = "system-library-rancher-monitoring"
)

var (
	APIVersion = types.APIVersion{
		Version: "v1",
		Group:   "monitoring.coreos.com",
		Path:    "/v3/project",
	}
)

func ClusterAlertManagerInfo() (appName, appTargetNamespace string) {
	return clusterLevelAlertManagerAppName, cattleNamespaceName
}

func ProjectMonitoringInfo(projectName string) (appName, appTargetNamespace string) {
	return projectLevelAppName, fmt.Sprintf("%s-%s", cattleNamespaceName, projectName)
}

func ClusterAlertManagerEndpoint() (headlessServiceName, namespace, port string) {
	return alertManagerHeadlessServiceName, cattleNamespaceName, "9093"
}

/*
OverwriteAppAnswersAndCatalogID Usage
## special key prefix
_tpl- [priority low] ->  regex ${value} = ${middle-prefix}#(${root1,root2,...}), then generate ${root*}.${middle-prefix} as prefix-key

## example

### input

	key 				 	|           			value

-----------------------------------------------------------------------------------------------
_tpl-Node_Selector       	     		| nodeSelector#(prometheus,grafana,exporter-kube-state)
_tpl-Storage_Class       	     		| persistence#(prometheus,grafana)
-----------------------------------------------------------------------------------------------
prometheus.retention				 	| 360h
exporter-node.ports.metrics.port	 	| 9100
grafana.persistence.enabled             | false
nodeSelector.region		 				| region-a
nodeSelector.zone         				| zone-b
persistence.enabled       				| true
persistence.storageClass  				| default
persistence.accessMode    				| ReadWriteOnce
persistence.size          				| 50Gi

### output

	key 				 	|           			value

-----------------------------------------------------------------------------------------------
prometheus.retention				 	| 360h
exporter-node.ports.metrics.port	 	| 9100
prometheus.nodeSelector.region		 	| region-a
prometheus.nodeSelector.zone         	| zone-b
grafana.nodeSelector.region		 		| region-a
grafana.nodeSelector.zone         		| zone-b
exporter-kube-state.nodeSelector.region	| region-a
exporter-kube-state.nodeSelector.zone   | zone-b
prometheus.persistence.enabled       	| true
prometheus.persistence.storageClass  	| default
prometheus.persistence.accessMode    	| ReadWriteOnce
prometheus.persistence.size          	| 50Gi
grafana.persistence.enabled       	 	| false         // can't overwrite by low priority
grafana.persistence.storageClass     	| default
grafana.persistence.accessMode       	| ReadWriteOnce
grafana.persistence.size             	| 50Gi
*/
