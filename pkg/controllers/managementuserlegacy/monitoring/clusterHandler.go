package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	app2 "github.com/rancher/rancher/pkg/app"
	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	kcluster "github.com/rancher/rancher/pkg/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rke/pki"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	exporterEtcdCertName = "exporter-etcd-cert"
	etcd                 = "etcd"
	controlplane         = "controlplane"
	windowsNode          = "windowsNode"
	creatorIDAnno        = "field.cattle.io/creatorId"
)

type etcdTLSConfig struct {
	certPath        string
	keyPath         string
	internalAddress string
}

type clusterHandler struct {
	clusterName          string
	cattleClustersClient mgmtv3.ClusterInterface
	cattleCatalogManager manager.CatalogManager
	agentEndpointsLister corev1.EndpointsLister
	app                  *appHandler
}

func (ch *clusterHandler) sync(key string, cluster *mgmtv3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name != ch.clusterName {
		return cluster, nil
	}

	if !cluster.Spec.Internal && !v32.ClusterConditionAgentDeployed.IsTrue(cluster) {
		return cluster, nil
	}

	clusterTag := getClusterTag(cluster)
	src := cluster
	cpy := src.DeepCopy()

	err := ch.doSync(cpy)
	if !reflect.DeepEqual(cpy, src) {
		updated, updateErr := ch.cattleClustersClient.Update(cpy)
		if updateErr != nil {
			return updated, errors.Wrapf(updateErr, "failed to update Cluster %s", clusterTag)
		}

		cpy = updated
	}

	return cpy, err
}

func (ch *clusterHandler) doSync(cluster *mgmtv3.Cluster) error {
	appName, appTargetNamespace := monitoring.ClusterMonitoringInfo()

	if cluster.Spec.EnableClusterMonitoring {
		appProjectName, err := ch.ensureAppProjectName(cluster.Name, appTargetNamespace)
		if err != nil {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to ensure monitoring project name")
		}

		var etcdTLSConfigs []*etcdTLSConfig
		var systemComponentMap map[string][]string
		if isRkeCluster(cluster) {
			if etcdTLSConfigs, err = ch.deployEtcdCert(cluster.Name, appTargetNamespace); err != nil {
				v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
				v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
				return errors.Wrap(err, "failed to deploy etcd cert")
			}
			if systemComponentMap, err = ch.getExporterEndpoint(); err != nil {
				v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
				v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
				return errors.Wrap(err, "failed to get exporter endpoint")
			}
		}

		_, err = ch.deployApp(appName, appTargetNamespace, appProjectName, cluster, etcdTLSConfigs, systemComponentMap)
		if err != nil {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to deploy monitoring")
		}

		if cluster.Status.MonitoringStatus == nil {
			cluster.Status.MonitoringStatus = &v32.MonitoringStatus{}
		}

		isReady, err := ch.isPrometheusReady(cluster)
		if err != nil {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return err
		}
		if !isReady {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, "prometheus is not ready")
			return nil
		}

		cluster.Status.MonitoringStatus.GrafanaEndpoint = fmt.Sprintf("/k8s/clusters/%s/api/v1/namespaces/%s/services/http:access-grafana:80/proxy/", cluster.Name, appTargetNamespace)

		_, err = ConditionMetricExpressionDeployed.DoUntilTrue(cluster.Status.MonitoringStatus, func() (status *v32.MonitoringStatus, e error) {
			return status, ch.deployMetrics(cluster)
		})
		if err != nil {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to deploy monitoring metrics")
		}

		v32.ClusterConditionMonitoringEnabled.True(cluster)
		v32.ClusterConditionMonitoringEnabled.Message(cluster, "")
	} else if enabledStatus := v32.ClusterConditionMonitoringEnabled.GetStatus(cluster); enabledStatus != "" && enabledStatus != "False" {
		if err := ch.app.withdrawApp(cluster.Name, appName, appTargetNamespace); err != nil {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to withdraw monitoring")
		}

		if err := ch.withdrawMetrics(cluster); err != nil {
			v32.ClusterConditionMonitoringEnabled.Unknown(cluster)
			v32.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to withdraw monitoring metrics")
		}

		v32.ClusterConditionMonitoringEnabled.False(cluster)
		v32.ClusterConditionMonitoringEnabled.Message(cluster, "")

		cluster.Status.MonitoringStatus = nil
	}

	return nil
}

func (ch *clusterHandler) ensureAppProjectName(clusterID, appTargetNamespace string) (string, error) {
	creator, err := ch.app.systemAccountManager.GetSystemUser(ch.clusterName)
	if err != nil {
		return "", err
	}

	appDeployProjectID, err := app2.GetSystemProjectID(clusterID, ch.app.projectLister)
	if err != nil {
		return "", err
	}

	appProjectName, err := app2.EnsureAppProjectName(ch.app.agentNamespaceClient, appDeployProjectID, clusterID, appTargetNamespace, creator.Name)
	if err != nil {
		return "", err
	}

	return appProjectName, nil
}

func (ch *clusterHandler) deployEtcdCert(clusterName, appTargetNamespace string) ([]*etcdTLSConfig, error) {
	var etcdTLSConfigs []*etcdTLSConfig

	rkeCertSecretName := "c-" + clusterName
	systemNamespace := "cattle-system"
	sec, err := ch.app.cattleSecretClient.GetNamespaced(systemNamespace, rkeCertSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get %s:%s in deploy etcd cert to prometheus", systemNamespace, rkeCertSecretName)
	}

	var data kcluster.Cluster
	if err = json.Unmarshal(sec.Data["cluster"], &data); err != nil {
		return nil, errors.Wrapf(err, "failed to decode secret %s:%s to get etcd cert", systemNamespace, rkeCertSecretName)
	}

	crts := make(map[string]map[string]string)
	if err = json.Unmarshal([]byte(data.Metadata["Certs"]), &crts); err != nil {
		return nil, errors.Wrapf(err, "failed to decode secret %s:%s cert data to get etcd cert", systemNamespace, rkeCertSecretName)
	}

	secretData := make(map[string][]byte)
	for k, v := range crts {
		if strings.HasPrefix(k, pki.EtcdCertName) {
			certName := getCertName(k)
			keyName := getKeyName(k)
			tlsConfig := etcdTLSConfig{
				internalAddress: k,
				certPath:        getSecretPath(exporterEtcdCertName, certName),
				keyPath:         getSecretPath(exporterEtcdCertName, keyName),
			}
			etcdTLSConfigs = append(etcdTLSConfigs, &tlsConfig)

			secretData[certName] = []byte(v["CertPEM"])
			secretData[keyName] = []byte(v["KeyPEM"])
		}
	}

	agentSecretClient := ch.app.agentSecretClient
	oldSec, err := agentSecretClient.GetNamespaced(appTargetNamespace, exporterEtcdCertName, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		secret := &k8scorev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      exporterEtcdCertName,
				Namespace: appTargetNamespace,
			},
			Data: secretData,
		}
		if _, err = agentSecretClient.Create(secret); err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		return etcdTLSConfigs, nil
	}

	newSec := oldSec.DeepCopy()
	newSec.Data = secretData
	_, err = agentSecretClient.Update(newSec)
	return etcdTLSConfigs, err
}

func (ch *clusterHandler) getExporterEndpoint() (map[string][]string, error) {
	endpointMap := make(map[string][]string)
	etcdLablels := labels.Set{
		"node-role.kubernetes.io/etcd": "true",
	}
	controlplaneLabels := labels.Set{
		"node-role.kubernetes.io/controlplane": "true",
	}
	windowsNodeLabels := labels.Set{
		"kubernetes.io/os": "windows",
	}

	agentNodeLister := ch.app.agentNodeClient.Controller().Lister()
	etcdNodes, err := agentNodeLister.List(metav1.NamespaceAll, etcdLablels.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get etcd nodes")
	}
	for _, v := range etcdNodes {
		endpointMap[etcd] = append(endpointMap[etcd], node.GetNodeInternalAddress(v))
	}

	controlplaneNodes, err := agentNodeLister.List(metav1.NamespaceAll, controlplaneLabels.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get controlplane nodes")
	}
	for _, v := range controlplaneNodes {
		endpointMap[controlplane] = append(endpointMap[controlplane], node.GetNodeInternalAddress(v))
	}

	windowsNodes, err := agentNodeLister.List(metav1.NamespaceAll, windowsNodeLabels.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get windows nodes")
	}
	for _, v := range windowsNodes {
		endpointMap[windowsNode] = append(endpointMap[windowsNode], node.GetNodeInternalAddress(v))
	}
	return endpointMap, nil
}

func (ch *clusterHandler) deployApp(appName, appTargetNamespace string, appProjectName string, cluster *mgmtv3.Cluster, etcdTLSConfig []*etcdTLSConfig, systemComponentMap map[string][]string) (map[string]string, error) {
	_, appDeployProjectID := ref.Parse(appProjectName)
	clusterAlertManagerSvcName, clusterAlertManagerSvcNamespaces, clusterAlertManagerPort := monitoring.ClusterAlertManagerEndpoint()
	optionalAppAnswers := map[string]string{
		"exporter-kube-state.enabled":    "true",
		"exporter-kubelets.enabled":      "true",
		"exporter-kubernetes.enabled":    "true",
		"exporter-node.enabled":          "true",
		"exporter-fluentd.enabled":       "true",
		"grafana.persistence.enabled":    "false",
		"prometheus.persistence.enabled": "false",
	}
	mustAppAnswers := map[string]string{
		"enabled":                   "false",
		"exporter-coredns.apiGroup": monitoring.APIVersion.Group,
		"exporter-kube-controller-manager.enabled":  "false",
		"exporter-kube-controller-manager.apiGroup": monitoring.APIVersion.Group,
		"exporter-kube-dns.apiGroup":                monitoring.APIVersion.Group,
		"exporter-kube-etcd.enabled":                "false",
		"exporter-kube-etcd.apiGroup":               monitoring.APIVersion.Group,
		"exporter-kube-scheduler.enabled":           "false",
		"exporter-kube-scheduler.apiGroup":          monitoring.APIVersion.Group,
		"exporter-kube-state.apiGroup":              monitoring.APIVersion.Group,
		"exporter-kubelets.apiGroup":                monitoring.APIVersion.Group,
		"exporter-kubernetes.apiGroup":              monitoring.APIVersion.Group,
		"exporter-node.apiGroup":                    monitoring.APIVersion.Group,
		"exporter-fluentd.apiGroup":                 monitoring.APIVersion.Group,
		"grafana.enabled":                           "true",
		"grafana.apiGroup":                          monitoring.APIVersion.Group,
		"grafana.serviceAccountName":                appName,
		"prometheus.enabled":                        "true",
		"prometheus.apiGroup":                       monitoring.APIVersion.Group,
		"prometheus.externalLabels.prometheus_from": cluster.Spec.DisplayName,
		"prometheus.serviceAccountNameOverride":     appName,
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].targets[0]":          fmt.Sprintf("%s.%s:%s", clusterAlertManagerSvcName, clusterAlertManagerSvcNamespaces, clusterAlertManagerPort),
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.level":        "cluster",
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.cluster_id":   cluster.Name,
		"prometheus.additionalAlertManagerConfigs[0].static_configs[0].labels.cluster_name": cluster.Spec.DisplayName,
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].key":                nslabels.ProjectIDFieldLabel,
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].operator":           "In",
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].values[0]":          appDeployProjectID,
		"prometheus.ruleNamespaceSelector.matchExpressions[0].key":                          nslabels.ProjectIDFieldLabel,
		"prometheus.ruleNamespaceSelector.matchExpressions[0].operator":                     "In",
		"prometheus.ruleNamespaceSelector.matchExpressions[0].values[0]":                    appDeployProjectID,
		"prometheus.ruleSelector.matchExpressions[0].key":                                   monitoring.CattlePrometheusRuleLabelKey,
		"prometheus.ruleSelector.matchExpressions[0].operator":                              "In",
		"prometheus.ruleSelector.matchExpressions[0].values[0]":                             monitoring.CattleAlertingPrometheusRuleLabelValue,
		"prometheus.ruleSelector.matchExpressions[0].values[1]":                             monitoring.CattleMonitoringPrometheusRuleLabelValue,
	}

	appAnswers, appAnswersSetString, appCatalogID, err := monitoring.OverwriteAppAnswersAndCatalogID(
		optionalAppAnswers,
		map[string]string{},
		cluster.Annotations,
		ch.app.catalogTemplateLister,
		ch.cattleCatalogManager,
		ch.clusterName,
	)
	if err != nil {
		return nil, err
	}

	// cannot overwrite mustAppAnswers
	for mustKey, mustVal := range mustAppAnswers {
		appAnswers[mustKey] = mustVal
		delete(appAnswersSetString, mustKey)
	}

	if systemComponentMap != nil {
		if etcdEndpoints, ok := systemComponentMap[etcd]; ok {
			appAnswers["prometheus.secrets[0]"] = exporterEtcdCertName
			appAnswers["exporter-kube-etcd.enabled"] = "true"
			appAnswers["exporter-kube-etcd.ports.metrics.port"] = "2379"
			sort.Strings(etcdEndpoints)
			for k, v := range etcdEndpoints {
				key := fmt.Sprintf("exporter-kube-etcd.endpoints[%d]", k)
				appAnswers[key] = v
			}

			if etcdTLSConfig != nil {
				sort.Slice(etcdTLSConfig, func(i, j int) bool {
					return etcdTLSConfig[i].internalAddress < etcdTLSConfig[j].internalAddress
				})
				appAnswers["exporter-kube-etcd.certFile"] = etcdTLSConfig[0].certPath
				appAnswers["exporter-kube-etcd.keyFile"] = etcdTLSConfig[0].keyPath
			}
		}

		if controlplaneEndpoints, ok := systemComponentMap[controlplane]; ok {
			appAnswers["exporter-kube-scheduler.enabled"] = "true"
			appAnswers["exporter-kube-controller-manager.enabled"] = "true"
			sort.Strings(controlplaneEndpoints)
			for k, v := range controlplaneEndpoints {
				key1 := fmt.Sprintf("exporter-kube-scheduler.endpoints[%d]", k)
				key2 := fmt.Sprintf("exporter-kube-controller-manager.endpoints[%d]", k)
				appAnswers[key1] = v
				appAnswers[key2] = v
			}
		}

		if windowsNodeEndpoints, ok := systemComponentMap[windowsNode]; ok {
			appAnswers["exporter-node-windows.enabled"] = "true"
			if port, ok := appAnswers["exporter-node.ports.metrics.port"]; ok {
				appAnswers["exporter-node-windows.ports.metrics.port"] = port
			}
			sort.Strings(windowsNodeEndpoints)
			for k, v := range windowsNodeEndpoints {
				key := fmt.Sprintf("exporter-node-windows.endpoints[%d]", k)
				appAnswers[key] = v
			}
		}
	}

	creator, err := ch.app.systemAccountManager.GetSystemUser(ch.clusterName)
	if err != nil {
		return nil, err
	}

	app := &v3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{creatorIDAnno: creator.Name},
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, appProjectName, monitoring.ClusterLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: v33.AppSpec{
			Answers:          appAnswers,
			AnswersSetString: appAnswersSetString,
			Description:      "Rancher Cluster Monitoring",
			ExternalID:       appCatalogID,
			ProjectName:      appProjectName,
			TargetNamespace:  appTargetNamespace,
		},
	}

	// redeploy cluster-monitoring App forcibly if its workload doesn't exist
	var forceRedeploy bool
	appWorkload, err := ch.app.agentStatefulSetLister.Get(appTargetNamespace, fmt.Sprintf("prometheus-%s", appName))
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, "failed to get statefulset %s/prometheus-%s", appTargetNamespace, appName)
	}
	if appWorkload == nil || appWorkload.Name == "" || appWorkload.DeletionTimestamp != nil {
		forceRedeploy = true
	}

	_, err = app2.DeployApp(ch.app.cattleAppClient, appDeployProjectID, app, forceRedeploy)
	if err != nil {
		return nil, err
	}

	return appAnswers, nil
}

func getClusterTag(cluster *mgmtv3.Cluster) string {
	return fmt.Sprintf("%s(%s)", cluster.Name, cluster.Spec.DisplayName)
}

func isRkeCluster(cluster *mgmtv3.Cluster) bool {
	return cluster.Status.Driver == v32.ClusterDriverRKE
}

func getSecretPath(secretName, name string) string {
	return fmt.Sprintf("/etc/prometheus/secrets/%s/%s", secretName, name)
}

func getCertName(name string) string {
	return fmt.Sprintf("%s.pem", name)
}

func getKeyName(name string) string {
	return fmt.Sprintf("%s-key.pem", name)
}

func (ch *clusterHandler) deployMetrics(cluster *mgmtv3.Cluster) error {
	clusterName := cluster.Name

	for _, metric := range preDefinedClusterMetrics {
		newObj := metric.DeepCopy()
		newObj.Namespace = clusterName

		_, err := ch.app.cattleMonitorMetricClient.Create(newObj)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	for _, graph := range preDefinedClusterGraph {
		newObj := graph.DeepCopy()
		newObj.Namespace = clusterName

		_, err := ch.app.cattleClusterGraphClient.Create(newObj)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func (ch *clusterHandler) withdrawMetrics(cluster *mgmtv3.Cluster) error {
	clusterName := cluster.Name

	for _, metric := range preDefinedClusterMetrics {
		err := ch.app.cattleMonitorMetricClient.DeleteNamespaced(clusterName, metric.Name, &metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	for _, graph := range preDefinedClusterGraph {
		err := ch.app.cattleClusterGraphClient.DeleteNamespaced(clusterName, graph.Name, &metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (ch *clusterHandler) isPrometheusReady(cluster *mgmtv3.Cluster) (bool, error) {
	svcName, namespace, _ := monitoring.ClusterPrometheusEndpoint()

	endpoints, err := ch.agentEndpointsLister.Get(namespace, svcName)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get %s/%s endpoints", namespace, svcName)
	}

	if len(endpoints.Subsets) == 0 || len(endpoints.Subsets[0].Addresses) == 0 {
		return false, nil
	}

	return true, nil
}
