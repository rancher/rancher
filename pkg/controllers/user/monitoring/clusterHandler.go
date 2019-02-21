package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	kcluster "github.com/rancher/kontainer-engine/cluster"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/pki"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	k8scorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	exporterEtcdCertSecretName = "exporter-etcd-cert"
)

type tlsConfig struct {
	certPath        string
	keyPath         string
	internalAddress string
	name            string
}

type clusterHandler struct {
	clusterName          string
	cattleClustersClient mgmtv3.ClusterInterface
	app                  *appHandler
}

func (ch *clusterHandler) sync(key string, cluster *mgmtv3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name != ch.clusterName {
		return cluster, nil
	}

	if !mgmtv3.ClusterConditionAgentDeployed.IsTrue(cluster) {
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

	if err != nil {
		err = errors.Wrapf(err, "unable to sync Cluster %s", clusterTag)
	}

	return cpy, err
}

func (ch *clusterHandler) doSync(cluster *mgmtv3.Cluster) error {
	appName, appTargetNamespace := monitoring.ClusterMonitoringInfo()
	isRkeCluster := isRkeCluster(cluster)

	if cluster.Spec.EnableClusterMonitoring {
		appProjectName, err := ch.ensureAppProjectName(cluster.Name, appTargetNamespace)
		if err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to ensure monitoring project name")
		}

		appAnswers, err := ch.deployApp(appName, appTargetNamespace, appProjectName, cluster, isRkeCluster)
		if err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to deploy monitoring")
		}

		if err := ch.detectAppComponentsWhileInstall(appName, appTargetNamespace, cluster, appAnswers); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to detect the installation status of monitoring components")
		}

		mgmtv3.ClusterConditionMonitoringEnabled.True(cluster)
		mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, "")
	} else if cluster.Status.MonitoringStatus != nil {
		if err := ch.withdrawApp(cluster.Name, appName, appTargetNamespace, isRkeCluster); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to withdraw monitoring")
		}

		if err := ch.detectAppComponentsWhileUninstall(appName, appTargetNamespace, cluster); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to detect the uninstallation status of monitoring components")
		}

		mgmtv3.ClusterConditionMonitoringEnabled.False(cluster)
		mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, "")
	}

	return nil
}

func (ch *clusterHandler) ensureAppProjectName(clusterID, appTargetNamespace string) (string, error) {
	appDeployProjectID, err := monitoring.GetSystemProjectID(ch.app.cattleProjectClient)
	if err != nil {
		return "", err
	}

	appProjectName, err := monitoring.EnsureAppProjectName(ch.app.agentNamespaceClient, appDeployProjectID, clusterID, appTargetNamespace)
	if err != nil {
		return "", err
	}

	return appProjectName, nil
}

func (ch *clusterHandler) deployEtcdCert(clusterName, appTargetNamespace string) (*tlsConfig, error) {
	var config *tlsConfig

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

	newSecret := &k8scorev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterEtcdCertSecretName,
			Namespace: appTargetNamespace,
		},
		Data: make(map[string][]byte),
		Type: k8scorev1.SecretTypeTLS,
	}
	for k, v := range crts {
		if strings.HasPrefix(k, pki.EtcdCertName) {
			newSecret.Data[k8scorev1.TLSCertKey] = []byte(v["CertPEM"])
			newSecret.Data[k8scorev1.TLSPrivateKeyKey] = []byte(v["KeyPEM"])

			config = &tlsConfig{
				internalAddress: k,
				certPath:        fmt.Sprintf("/etc/exporter-rke/secrets/%s/%s", exporterEtcdCertSecretName, k8scorev1.TLSCertKey),
				keyPath:         fmt.Sprintf("/etc/exporter-rke/secrets/%s/%s", exporterEtcdCertSecretName, k8scorev1.TLSPrivateKeyKey),
				name:            exporterEtcdCertSecretName,
			}

			// taking one cert is enough
			break
		}
	}

	agentSecretClient := ch.app.agentSecretClient
	oldSecret, err := agentSecretClient.GetNamespaced(appTargetNamespace, exporterEtcdCertSecretName, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		if _, err := agentSecretClient.Create(newSecret); err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		return config, nil
	}

	if !reflect.DeepEqual(oldSecret.Data, newSecret.Data) {
		newSecret.ResourceVersion = oldSecret.ResourceVersion
		if _, err := agentSecretClient.Update(newSecret); err != nil {
			return nil, err
		}
	}

	return config, nil
}

func (ch *clusterHandler) deployApp(appName, appTargetNamespace string, appProjectName string, cluster *mgmtv3.Cluster, isRkeCluster bool) (map[string]string, error) {
	_, appDeployProjectID := ref.Parse(appProjectName)
	clusterAlertManagerSvcName, clusterAlertManagerSvcNamespaces, clusterAlertManagerPort := monitoring.ClusterAlertManagerEndpoint()

	optionalAppAnswers := map[string]string{
		"exporter-kube-state.enabled": "true",

		"exporter-kubelets.enabled": "true",

		"exporter-kubernetes.enabled": "true",

		"exporter-node.enabled": "true",

		"exporter-fluentd.enabled": "true",

		"grafana.persistence.enabled": "false",

		"prometheus.persistence.enabled": "false",
	}

	mustAppAnswers := map[string]string{
		"enabled": "false",

		"exporter-rke.enabled":  "false",
		"exporter-rke.apiGroup": monitoring.APIVersion.Group,

		"exporter-coredns.enabled":  "false",
		"exporter-coredns.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-dns.enabled":  "false",
		"exporter-kube-dns.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-state.apiGroup": monitoring.APIVersion.Group,

		"exporter-kubelets.apiGroup": monitoring.APIVersion.Group,

		"exporter-kubernetes.apiGroup": monitoring.APIVersion.Group,

		"exporter-node.apiGroup": monitoring.APIVersion.Group,

		"exporter-fluentd.apiGroup": monitoring.APIVersion.Group,

		"grafana.enabled":            "true",
		"grafana.apiGroup":           monitoring.APIVersion.Group,
		"grafana.serviceAccountName": appName,

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

	appAnswers := monitoring.OverwriteAppAnswers(optionalAppAnswers, cluster.Annotations)

	// cannot overwrite mustAppAnswers
	for mustKey, mustVal := range mustAppAnswers {
		appAnswers[mustKey] = mustVal
	}

	if isRkeCluster {
		appAnswers["exporter-rke.enabled"] = "true"

		// enable etcd scraping
		etcdTLSConfig, err := ch.deployEtcdCert(cluster.Name, appTargetNamespace)
		if err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return nil, errors.Wrap(err, "failed to deploy etcd cert for RKE")
		}
		if etcdTLSConfig != nil {
			appAnswers["exporter-rke.secrets[0]"] = etcdTLSConfig.name
			appAnswers["exporter-rke.etcd.port"] = "2379"
			appAnswers["exporter-rke.etcd.tlsConfig.certFile"] = etcdTLSConfig.certPath
			appAnswers["exporter-rke.etcd.tlsConfig.keyFile"] = etcdTLSConfig.keyPath
		}
	}

	appCatalogID := settings.SystemMonitoringCatalogID.Get()
	app := &v3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: monitoring.CopyCreatorID(nil, cluster.Annotations),
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, appProjectName, monitoring.ClusterLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: v3.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher Cluster Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	err := monitoring.DeployApp(ch.app.cattleAppClient, appDeployProjectID, app)
	if err != nil {
		return nil, err
	}

	return appAnswers, nil
}

func (ch *clusterHandler) withdrawApp(clusterID, appName, appTargetNamespace string, isRkeCluster bool) error {
	if isRkeCluster {
		err := ch.app.agentSecretClient.DeleteNamespaced(appTargetNamespace, exporterEtcdCertSecretName, &metav1.DeleteOptions{})
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return ch.app.withdrawApp(clusterID, appName, appTargetNamespace)
}

func (ch *clusterHandler) detectAppComponentsWhileInstall(appName, appTargetNamespace string, cluster *mgmtv3.Cluster, appAnswers map[string]string) error {
	if cluster.Status.MonitoringStatus == nil {
		cluster.Status.MonitoringStatus = &mgmtv3.MonitoringStatus{
			Conditions: []mgmtv3.MonitoringCondition{
				{Type: mgmtv3.ClusterConditionType(ConditionGrafanaDeployed), Status: k8scorev1.ConditionFalse},
				{Type: mgmtv3.ClusterConditionType(ConditionNodeExporterDeployed), Status: k8scorev1.ConditionFalse},
				{Type: mgmtv3.ClusterConditionType(ConditionKubeStateExporterDeployed), Status: k8scorev1.ConditionFalse},
				{Type: mgmtv3.ClusterConditionType(ConditionPrometheusDeployed), Status: k8scorev1.ConditionFalse},
				{Type: mgmtv3.ClusterConditionType(ConditionMetricExpressionDeployed), Status: k8scorev1.ConditionFalse},
			},
		}
	}
	monitoringStatus := cluster.Status.MonitoringStatus

	enabledExporterNode := convert.ToBool(appAnswers["exporter-node.enabled"])
	if enabledExporterNode {
		ConditionNodeExporterDeployed.Add(monitoringStatus)
	} else {
		ConditionNodeExporterDeployed.Del(monitoringStatus)
	}

	enabledExporterKubeState := convert.ToBool(appAnswers["exporter-kube-state.enabled"])
	if enabledExporterKubeState {
		ConditionKubeStateExporterDeployed.Add(monitoringStatus)
	} else {
		ConditionKubeStateExporterDeployed.Del(monitoringStatus)
	}

	checkers := make([]func() error, 0, len(monitoringStatus.Conditions))
	if !ConditionGrafanaDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isGrafanaDeployed(ch.app.agentDeploymentClient, appTargetNamespace, appName, monitoringStatus, cluster.Name)
		})
	}
	if enabledExporterNode && !ConditionNodeExporterDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isNodeExporterDeployed(ch.app.agentDaemonSetClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if enabledExporterKubeState && !ConditionKubeStateExporterDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isKubeStateExporterDeployed(ch.app.agentDeploymentClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionPrometheusDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isPrometheusDeployed(ch.app.agentStatefulSetClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionMetricExpressionDeployed.IsTrue(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isMetricExpressionDeployed(cluster.Name, ch.app.cattleClusterGraphClient, ch.app.cattleMonitorMetricClient, monitoringStatus)
		})
	}

	if len(checkers) == 0 {
		return nil
	}

	err := stream(checkers...)
	if err != nil {
		time.Sleep(5 * time.Second)
	}

	return err
}

func (ch *clusterHandler) detectAppComponentsWhileUninstall(appName, appTargetNamespace string, cluster *mgmtv3.Cluster) error {
	if cluster.Status.MonitoringStatus == nil {
		return nil
	}
	monitoringStatus := cluster.Status.MonitoringStatus

	checkers := make([]func() error, 0, len(monitoringStatus.Conditions))
	if !ConditionGrafanaDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isGrafanaWithdrew(ch.app.agentDeploymentClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionNodeExporterDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isNodeExporterWithdrew(ch.app.agentDaemonSetClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionKubeStateExporterDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isKubeStateExporterWithdrew(ch.app.agentDeploymentClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionPrometheusDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isPrometheusWithdrew(ch.app.agentStatefulSetClient, appTargetNamespace, appName, monitoringStatus)
		})
	}
	if !ConditionMetricExpressionDeployed.IsFalse(monitoringStatus) {
		checkers = append(checkers, func() error {
			return isMetricExpressionWithdrew(cluster.Name, ch.app.cattleClusterGraphClient, ch.app.cattleMonitorMetricClient, monitoringStatus)
		})
	}

	if len(checkers) == 0 {
		return nil
	}

	err := stream(checkers...)
	if err != nil {
		time.Sleep(5 * time.Second)
	}

	return err
}

func getClusterTag(cluster *mgmtv3.Cluster) string {
	return fmt.Sprintf("%s(%s)", cluster.Name, cluster.Spec.DisplayName)
}

func isRkeCluster(cluster *mgmtv3.Cluster) bool {
	return cluster.Status.Driver == mgmtv3.ClusterDriverRKE
}
