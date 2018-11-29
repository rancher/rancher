package monitoring

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rancher/norman/types"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Level string

const (
	SystemLevel  Level = "system"
	ClusterLevel Level = "cluster"
	ProjectLevel Level = "project"
)

const (
	CattleNamespaceName                              = "cattle-prometheus"
	CattleCreatorIDAnnotationKey                     = "field.cattle.io/creatorId"
	CattleProjectIDAnnotationKey                     = "field.cattle.io/projectId"
	CattleOverwriteMonitoringAppAnswersAnnotationKey = "field.cattle.io/overwriteMonitoringAppAnswers"
	CattleProjectIDLabelKey                          = "field.cattle.io/projectId"
	ClusterAppName                                   = "cluster-monitoring"
	ProjectAppName                                   = "project-monitoring"
)

const (
	// The label info of Namespace
	CattleMonitoringLabelKey = "monitoring.coreos.com"

	// The label info of App, RoleBinding
	appNameLabelKey            = CattleMonitoringLabelKey + "/appName"
	appTargetNamespaceLabelKey = CattleMonitoringLabelKey + "/appTargetNamespace"
	levelLabelKey              = CattleMonitoringLabelKey + "/level"

	// The names of App
	systemAppName           = "system-monitoring"
	metricExpressionAppName = "metric-expression"
	alertManagerAppName     = "cluster-alerting"

	// The headless service name of Prometheus
	alertmanagerHeadlessServiceName = "alertmanager-operated"
	prometheusHeadlessServiceName   = "prometheus-operated"

	// The label info of PrometheusRule
	CattlePrometheusRuleLabelKey           = "source"
	CattleAlertingPrometheusRuleLabelValue = "rancher-alert"
)

var (
	APIVersion = types.APIVersion{
		Version: "v1",
		Group:   "monitoring.coreos.com",
		Path:    "/v3/project",
	}

	tplRegexp = &templateRegexp{
		r: regexp.MustCompile(`(?P<middlePrefix>.+)#\((?P<roots>.+)\)`),
	}
)

func OwnedAppListOptions(appName, appTargetNamespace string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s, %s=%s", appNameLabelKey, appName, appTargetNamespaceLabelKey, appTargetNamespace),
	}
}

func OwnedLabels(appName, appTargetNamespace string, level Level) map[string]string {
	return map[string]string{
		appNameLabelKey:            appName,
		appTargetNamespaceLabelKey: appTargetNamespace,
		levelLabelKey:              string(level),
	}
}

func OwnedProjectListOptions(projectName string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", CattleProjectIDAnnotationKey, projectName),
	}
}

func SystemMonitoringInfo() (appName, appTargetNamespace string) {
	return systemAppName, CattleNamespaceName
}

func ClusterMonitoringInfo() (appName, appTargetNamespace string) {
	return ClusterAppName, CattleNamespaceName
}

func ClusterMonitoringMetricsInfo() (appName, appTargetNamespace string) {
	return metricExpressionAppName, CattleNamespaceName
}

func ClusterAlertManagerInfo() (appName, appTargetNamespace string) {
	return alertManagerAppName, CattleNamespaceName
}

func ProjectMonitoringInfo(project *mgmtv3.Project) (appName, appTargetNamespace string) {
	return ProjectAppName, fmt.Sprintf("%s-%s", CattleNamespaceName, project.Name)
}

func ClusterAlertManagerEndpoint() (headlessServiceName, namespace, port string) {
	return alertmanagerHeadlessServiceName, CattleNamespaceName, "9093"
}

func ClusterAlertManagerSecret() string {
	return fmt.Sprintf("alertmanager-%s", alertManagerAppName)
}

func ClusterPrometheusEndpoint() (headlessServiceName, namespace, port string) {
	return prometheusHeadlessServiceName, CattleNamespaceName, "9090"
}

func ProjectPrometheusEndpoint(project *mgmtv3.Project) (headlessServiceName, namespace string, port string) {
	return prometheusHeadlessServiceName, fmt.Sprintf("%s-%s", CattleNamespaceName, project.Name), "9090"
}

/**
## usage

_key-* [priority-0]	-> 	select ${value} as key
_tpl-* [priority-1] ->  regex ${value} = ${middle-prefix}#(${root1,root2,...}), then generate ${root*}.${middle-prefix} as prefix-key

## expample

### input
				key 				 	|           			value
-----------------------------------------------------------------------------------------------
_key-Data_Retention  					| prometheus.retention
_key-Node_Exporter_Host_Port    		| exporter-node.ports.metrics.port
_key-Grafana_Storage_Enabled			| grafana.persistence.enabled
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

### translate

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
grafana.persistence.enabled       	 	| false         // priority overwrite
grafana.persistence.storageClass     	| default
grafana.persistence.accessMode       	| ReadWriteOnce
grafana.persistence.size             	| 50Gi

*/
func OverwriteAppAnswers(rawAnswers map[string]string, overwriteAnswers map[string]string) map[string]string {
	keyRecord := make(map[string]struct{}, 8)

	for specialKey, value := range overwriteAnswers {
		if strings.HasPrefix(specialKey, "_key-") {
			actualKey := value
			keyRecord[actualKey] = struct{}{}
			rawAnswers[actualKey] = overwriteAnswers[actualKey]
			delete(overwriteAnswers, specialKey)
			delete(overwriteAnswers, actualKey)
		} else if strings.HasPrefix(specialKey, "_tpl-") {
			trr := tplRegexp.translate(value)
			for suffixKey, value := range overwriteAnswers {
				if strings.HasPrefix(suffixKey, trr.middlePrefix) {
					for _, prefixKey := range trr.roots {
						actualKey := fmt.Sprintf("%s.%s", prefixKey, suffixKey)
						if _, existed := keyRecord[actualKey]; existed {
							continue
						}

						keyRecord[actualKey] = struct{}{}
						rawAnswers[actualKey] = value
					}

					delete(overwriteAnswers, suffixKey)
				}
			}

			delete(overwriteAnswers, specialKey)
		}
	}

	return rawAnswers
}

type templateRegexpResult struct {
	middlePrefix string
	roots        []string
}

type templateRegexp struct {
	r *regexp.Regexp
}

func (tr *templateRegexp) translate(value string) *templateRegexpResult {
	captures := &templateRegexpResult{}

	match := tr.r.FindStringSubmatch(value)
	if match == nil {
		return captures
	}

	for i, name := range tr.r.SubexpNames() {
		if name == "middlePrefix" {
			captures.middlePrefix = match[i]
		} else if name == "roots" {
			roots := strings.Split(match[i], ",")
			for _, root := range roots {
				root = strings.TrimSpace(root)
				if len(root) != 0 {
					captures.roots = append(captures.roots, root)
				}
			}
		}

	}

	return captures
}

func ProjectMonitoringNamespace(projectName string) string {
	//projectName sample: p-xxx
	return fmt.Sprintf("cattle-prometheus-%s", projectName)
}
