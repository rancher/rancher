package monitoring

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalog/manager"
	cutils "github.com/rancher/rancher/pkg/catalog/utils"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	ns "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppLevel string

const (
	SystemLevel  AppLevel = "system"
	ClusterLevel AppLevel = "cluster"
	ProjectLevel AppLevel = "project"
)

const (
	istioNamespaceName                     = "istio-system"
	cattleNamespaceName                    = "cattle-prometheus"
	cattleCreatorIDAnnotationKey           = "field.cattle.io/creatorId"
	cattleOverwriteAppAnswersAnnotationKey = "field.cattle.io/overwriteAppAnswers"
)

const (
	//CattleMonitoringLabelKey The label info of Namespace
	cattleMonitoringLabelKey = "monitoring.coreos.com"

	// The label info of App, RoleBinding
	appNameLabelKey            = cattleMonitoringLabelKey + "/appName"
	appTargetNamespaceLabelKey = cattleMonitoringLabelKey + "/appTargetNamespace"
	appProjectIDLabelKey       = cattleMonitoringLabelKey + "/projectID"
	appClusterIDLabelKey       = cattleMonitoringLabelKey + "/clusterID"
	appLevelLabelKey           = cattleMonitoringLabelKey + "/level"

	// The names of App
	systemLevelAppName              = "monitoring-operator"
	clusterLevelAppName             = "cluster-monitoring"
	projectLevelAppName             = "project-monitoring"
	clusterLevelAlertManagerAppName = "cluster-alerting"
	IstioAppName                    = "cluster-istio"

	// The headless service name of Prometheus
	alertManagerHeadlessServiceName = "alertmanager-operated"
	prometheusHeadlessServiceName   = "prometheus-operated"

	// The service name of istio prometheus
	istioPrometheusServiceName = "prometheus"

	istioMonitoringTypeClusterMonitoring = "cluster-monitoring"
	istioMonitoringTypesBuiltIn          = "built-in"
	istioMonitoringTypesCustom           = "custom"

	//CattlePrometheusRuleLabelKey The label info of PrometheusRule
	CattlePrometheusRuleLabelKey             = "source"
	CattleAlertingPrometheusRuleLabelValue   = "rancher-alert"
	CattleMonitoringPrometheusRuleLabelValue = "rancher-monitoring"
	RancherMonitoringTemplateName            = "system-library-rancher-monitoring"

	monitoringTemplateName = "rancher-monitoring"
	webhookSecreteName     = "webhook-receiver"
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

func OwnedAppListOptions(clusterID, appName, appTargetNamespace string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s, %s=%s, %s=%s", appClusterIDLabelKey, clusterID, appNameLabelKey, appName, appTargetNamespaceLabelKey, appTargetNamespace),
	}
}

func CopyCreatorID(toAnnotations, fromAnnotations map[string]string) map[string]string {
	if val, exist := fromAnnotations[cattleCreatorIDAnnotationKey]; exist {
		if toAnnotations == nil {
			toAnnotations = make(map[string]string, 2)
		}

		toAnnotations[cattleCreatorIDAnnotationKey] = val
	}

	return toAnnotations
}

func AppendAppOverwritingAnswers(toAnnotations map[string]string, appOverwriteAnswers string) map[string]string {
	if len(strings.TrimSpace(appOverwriteAnswers)) != 0 {
		if toAnnotations == nil {
			toAnnotations = make(map[string]string, 2)
		}

		toAnnotations[cattleOverwriteAppAnswersAnnotationKey] = appOverwriteAnswers
	}

	return toAnnotations
}

func OwnedLabels(appName, appTargetNamespace, appProjectName string, level AppLevel) map[string]string {
	clusterID, projectID := ref.Parse(appProjectName)

	return map[string]string{
		appNameLabelKey:            appName,
		appTargetNamespaceLabelKey: appTargetNamespace,
		appProjectIDLabelKey:       projectID,
		appClusterIDLabelKey:       clusterID,
		appLevelLabelKey:           string(level),
	}
}

func IstioPrometheusEndpoint(answers map[string]string) (serviceName, namespace, port string) {
	if answers["global.monitoring.type"] == istioMonitoringTypeClusterMonitoring {
		return prometheusHeadlessServiceName, cattleNamespaceName, "9090"
	}
	return istioPrometheusServiceName, istioNamespaceName, "9090"
}

func SystemMonitoringInfo() (appName, appTargetNamespace string) {
	return systemLevelAppName, cattleNamespaceName
}

func ClusterMonitoringInfo() (appName, appTargetNamespace string) {
	return clusterLevelAppName, cattleNamespaceName
}

func ClusterAlertManagerInfo() (appName, appTargetNamespace string) {
	return clusterLevelAlertManagerAppName, cattleNamespaceName
}

func SecretWebhook() (secretName, appTargetNamespace string) {
	return webhookSecreteName, cattleNamespaceName
}

func ProjectMonitoringInfo(projectName string) (appName, appTargetNamespace string) {
	return projectLevelAppName, fmt.Sprintf("%s-%s", cattleNamespaceName, projectName)
}

func ClusterAlertManagerEndpoint() (headlessServiceName, namespace, port string) {
	return alertManagerHeadlessServiceName, cattleNamespaceName, "9093"
}

func ClusterPrometheusEndpoint() (headlessServiceName, namespace, port string) {
	return prometheusHeadlessServiceName, cattleNamespaceName, "9090"
}

func ProjectPrometheusEndpoint(projectName string) (headlessServiceName, namespace, port string) {
	return prometheusHeadlessServiceName, fmt.Sprintf("%s-%s", cattleNamespaceName, projectName), "9090"
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
func OverwriteAppAnswersAndCatalogID(
	rawAnswers,
	rawAnswersSetString map[string]string,
	annotations map[string]string,
	catalogTemplateLister mgmtv3.CatalogTemplateLister,
	catalogManager manager.CatalogManager,
	clusterName string,
) (map[string]string, map[string]string, string, error) {
	monitoringInput := GetMonitoringInput(annotations)
	resolveSpecialAnswersKeys(rawAnswers, monitoringInput.Answers)
	resolveSpecialAnswersKeys(rawAnswersSetString, monitoringInput.AnswersSetString)
	catalogID, err := GetMonitoringCatalogID(monitoringInput.Version, catalogTemplateLister, catalogManager, clusterName)
	return rawAnswers, rawAnswersSetString, catalogID, err
}

func resolveSpecialAnswersKeys(rawAnswers, answers map[string]string) {
	for specialKey, value := range answers {
		if strings.HasPrefix(specialKey, "_tpl-") {
			trr := tplRegexp.translate(value)
			for suffixKey, value := range answers {
				if strings.HasPrefix(suffixKey, trr.middlePrefix) {
					for _, prefixKey := range trr.roots {
						actualKey := fmt.Sprintf("%s.%s", prefixKey, suffixKey)
						rawAnswers[actualKey] = value
					}
					delete(answers, suffixKey)
				}
			}
			delete(answers, specialKey)
		}
	}
	for key, value := range answers {
		rawAnswers[key] = value
	}
}

func GetMonitoringCatalogID(version string, catalogTemplateLister mgmtv3.CatalogTemplateLister, catalogManager manager.CatalogManager, clusterName string) (string, error) {
	if version == "" {
		template, err := catalogTemplateLister.Get(ns.GlobalNamespace, RancherMonitoringTemplateName)
		if err != nil {
			return "", err
		}

		templateVersion, err := catalogManager.LatestAvailableTemplateVersion(template, clusterName)
		if err != nil {
			return "", err
		}
		version = templateVersion.Version
	}
	return fmt.Sprintf(cutils.CatalogExternalIDFormat, cutils.SystemLibraryName, monitoringTemplateName, version), nil
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

func GetMonitoringInput(annotations map[string]string) v32.MonitoringInput {
	overwritingAppAnswers := annotations[cattleOverwriteAppAnswersAnnotationKey]
	if len(overwritingAppAnswers) != 0 {
		var appOverwriteInput v32.MonitoringInput
		err := json.Unmarshal([]byte(overwritingAppAnswers), &appOverwriteInput)
		if err == nil {
			return appOverwriteInput
		}
		logrus.Errorf("failed to parse app overwrite input from %q, %v", overwritingAppAnswers, err)
	}

	return v32.MonitoringInput{}
}
