package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	kcluster "github.com/rancher/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/monitoring"
	nodeutil "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/pki"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	corev1 "github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	k8scorev1 "k8s.io/api/core/v1"
	k8srbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	exporterEtcdCertName = "exporter-etcd-cert"
	etcd                 = "etcd"
	controlplane         = "controlplane"
)

type etcdTLSConfig struct {
	certPath        string
	keyPath         string
	internalAddress string
}

type appHandler struct {
	cattleTemplateVersionsGetter mgmtv3.TemplateVersionsGetter
	cattleProjectsGetter         mgmtv3.ProjectsGetter
	cattleAppsGetter             projectv3.AppsGetter
	cattleCoreClient             corev1.Interface
	agentRBACClient              rbacv1.Interface
	agentCoreClient              corev1.Interface
	agentWorkloadsClient         appsv1beta2.Interface
}

type clusterHandler struct {
	ctx                  context.Context
	clusterName          string
	cattleClustersClient mgmtv3.ClusterInterface
	clusterGraph         mgmtv3.ClusterMonitorGraphInterface
	monitorMetrics       mgmtv3.MonitorMetricInterface
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

	err := ch.syncSystemMonitor(cpy)
	if err == nil {
		err = ch.syncClusterMonitoring(cpy)
	}

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

func (ch *clusterHandler) syncSystemMonitor(cluster *mgmtv3.Cluster) (err error) {
	return monitoring.SyncServiceMonitor(cluster, ch.app.agentCoreClient, ch.app.agentRBACClient, ch.app.cattleAppsGetter, ch.app.cattleProjectsGetter, ch.app.cattleTemplateVersionsGetter.TemplateVersions(metav1.NamespaceAll))
}

func (ch *clusterHandler) syncClusterMonitoring(cluster *mgmtv3.Cluster) error {
	appName, appTargetNamespace := monitoring.ClusterMonitoringInfo()

	if cluster.Spec.EnableClusterMonitoring {
		appProjectName, err := ch.app.ensureClusterMonitoringProjectName(cluster.Name, appTargetNamespace)
		if err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to ensure monitoring project name")
		}

		var etcdTLSConfigs []*etcdTLSConfig
		var systemComponentMap map[string][]string
		if isRkeCluster(cluster) {
			if etcdTLSConfigs, err = ch.app.deployEtcdCert(cluster.Name, appTargetNamespace); err != nil {
				mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
				mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
				return errors.Wrap(err, "failed to deploy etcd cert")
			}
			if systemComponentMap, err = ch.app.getExporterEndpoint(); err != nil {
				mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
				mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
				return errors.Wrap(err, "failed to get exporter endpoint")
			}
		}

		appServiceAccountName, err := ch.app.grantClusterMonitoringPermissions(appName, appTargetNamespace)
		if err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to grant monitoring permissions")
		}

		if err := ch.app.deployClusterMonitoringApp(appName, appTargetNamespace, appServiceAccountName, appProjectName, cluster, etcdTLSConfigs, systemComponentMap); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to deploy monitoring")
		}

		if err := ch.detectMonitoringComponentsWhileInstall(appName, appTargetNamespace, cluster); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to detect the installation status of monitoring components")
		}

		mgmtv3.ClusterConditionMonitoringEnabled.True(cluster)
		mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, "")
	} else if cluster.Status.MonitoringStatus != nil {
		if err := ch.app.withdrawApp(appName, appTargetNamespace); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to withdraw monitoring")
		}

		if err := ch.detectMonitoringComponentsWhileUninstall(appName, appTargetNamespace, cluster); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to detect the uninstallation status of monitoring components")
		}

		appServiceAccountName := appName
		appClusterRoleName := appName
		appClusterRoleBindingName := appClusterRoleName + "-binding"
		if err := ch.app.revokePermissions(appServiceAccountName, appClusterRoleName, appClusterRoleBindingName, appTargetNamespace); err != nil {
			mgmtv3.ClusterConditionMonitoringEnabled.Unknown(cluster)
			mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, err.Error())
			return errors.Wrap(err, "failed to revoke monitoring permissions")
		}

		mgmtv3.ClusterConditionMonitoringEnabled.False(cluster)
		mgmtv3.ClusterConditionMonitoringEnabled.Message(cluster, "")
	}

	return nil
}

func (ch *clusterHandler) detectMonitoringComponentsWhileInstall(appName, appTargetNamespace string, cluster *mgmtv3.Cluster) error {
	time.Sleep(5 * time.Second)

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

	return stream(
		func() error {
			return isGrafanaDeployed(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus, cluster.Name)
		},
		func() error {
			return isNodeExporterDeployed(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
		func() error {
			return isKubeStateExporterDeployed(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
		func() error {
			return isPrometheusDeployed(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
		func() error {
			return isMetricExpressionDeployed(cluster.Name, ch.clusterGraph, ch.monitorMetrics, monitoringStatus)
		},
	)
}

func (ch *clusterHandler) detectMonitoringComponentsWhileUninstall(appName, appTargetNamespace string, cluster *mgmtv3.Cluster) error {
	if cluster.Status.MonitoringStatus == nil {
		return nil
	}

	time.Sleep(5 * time.Second)

	monitoringStatus := cluster.Status.MonitoringStatus

	return stream(
		func() error {
			return isMetricExpressionWithdrew(cluster.Name, ch.clusterGraph, ch.monitorMetrics, monitoringStatus)
		},
		func() error {
			return isPrometheusWithdrew(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
		func() error {
			return isKubeStateExporterWithdrew(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
		func() error {
			return isNodeExporterWithdrew(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
		func() error {
			return isGrafanaWithdrew(ch.app.agentWorkloadsClient, appTargetNamespace, appName, monitoringStatus)
		},
	)
}

func (ah *appHandler) deployEtcdCert(clusterName, appTargetNamespace string) ([]*etcdTLSConfig, error) {
	var etcdTLSConfigs []*etcdTLSConfig

	rkeCertSecretName := "c-" + clusterName
	systemNamespace := "cattle-system"
	sec, err := ah.cattleCoreClient.Secrets(systemNamespace).Get(rkeCertSecretName, metav1.GetOptions{})
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

	agentUserSecretsClient := ah.agentCoreClient.Secrets(appTargetNamespace)
	oldSec, err := agentUserSecretsClient.Get(exporterEtcdCertName, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		secret := newSecret(exporterEtcdCertName, appTargetNamespace, secretData)
		if _, err = agentUserSecretsClient.Create(secret); err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		return etcdTLSConfigs, nil
	}

	newSec := oldSec.DeepCopy()
	newSec.Data = secretData
	_, err = agentUserSecretsClient.Update(newSec)
	return etcdTLSConfigs, err
}

func newSecret(name, namespace string, data map[string][]byte) *k8scorev1.Secret {
	return &k8scorev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func (ah *appHandler) getExporterEndpoint() (map[string][]string, error) {
	endpointMap := make(map[string][]string)
	etcdLablels := labels.Set{
		"node-role.kubernetes.io/etcd": "true",
	}
	controlplaneLabels := labels.Set{
		"node-role.kubernetes.io/controlplane": "true",
	}

	agentNodeLister := ah.agentCoreClient.Nodes(metav1.NamespaceAll).Controller().Lister()
	etcdNodes, err := agentNodeLister.List(metav1.NamespaceAll, etcdLablels.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get etcd nodes")
	}
	for _, v := range etcdNodes {
		endpointMap[etcd] = append(endpointMap[etcd], nodeutil.GetNodeInternalAddress(v))
	}

	controlplaneNodes, err := agentNodeLister.List(metav1.NamespaceAll, controlplaneLabels.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get controlplane nodes")
	}
	for _, v := range controlplaneNodes {
		endpointMap[controlplane] = append(endpointMap[controlplane], nodeutil.GetNodeInternalAddress(v))
	}

	return endpointMap, nil
}

func (ah *appHandler) ensureClusterMonitoringProjectName(clusterID, appTargetNamespace string) (string, error) {
	appDeployProjectID, err := monitoring.GetSystemProjectID(ah.cattleProjectsGetter.Projects(clusterID))
	if err != nil {
		return "", err
	}

	agentNamespacesClient := ah.agentCoreClient.Namespaces(metav1.NamespaceAll)
	appProjectName, err := monitoring.EnsureAppProjectName(agentNamespacesClient, appDeployProjectID, clusterID, appTargetNamespace)
	if err != nil {
		return "", err
	}

	return appProjectName, nil
}

func (ah *appHandler) grantClusterMonitoringPermissions(appName, appTargetNamespace string) (string, error) {
	appServiceAccountName := appName
	appClusterRoleName := appServiceAccountName
	appClusterRoleBindingName := appServiceAccountName + "-binding"
	ownedLabels := monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ClusterLevel)

	err := stream(
		// detect ServiceAccount (the name as same as App)
		func() error {
			appServiceAccount, err := ah.agentCoreClient.ServiceAccounts(appTargetNamespace).Get(appServiceAccountName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ServiceAccount in %q Namespace", appServiceAccountName, appTargetNamespace)
			}
			if appServiceAccount.Name == appServiceAccountName {
				if appServiceAccount.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ServiceAccount in %q Namespace is still on terminating", appServiceAccountName, appTargetNamespace))
				}
			} else {
				appServiceAccount = &k8scorev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appServiceAccountName,
						Namespace: appTargetNamespace,
						Labels:    ownedLabels,
					},
				}

				if _, err := ah.agentCoreClient.ServiceAccounts(appTargetNamespace).Create(appServiceAccount); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ServiceAccount in %q Namespace", appServiceAccountName, appTargetNamespace)
				}
			}

			return nil
		},

		// detect ClusterRole (the name as same as App)
		func() error {
			appClusterRole, err := ah.agentRBACClient.ClusterRoles(metav1.NamespaceAll).Get(appClusterRoleName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ClusterRole", appClusterRoleName)
			}

			rules := []k8srbacv1.PolicyRule{
				{
					NonResourceURLs: []string{"/metrics"},
					Verbs:           []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"services", "endpoints", "pods", "nodes"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes/metrics"}, // kubelet
					Verbs:     []string{"get"},
				},
				// for Node Exporter
				{
					APIGroups: []string{"authentication.k8s.io"},
					Resources: []string{"tokenreviews"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"authorization.k8s.io"},
					Resources: []string{"subjectaccessreviews"},
					Verbs:     []string{"create"},
				},
				// for Kube-state Exporter
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces", "nodes", "pods", "services", "resourcequotas", "replicationcontrollers", "limitranges", "persistentvolumeclaims", "persistentvolumes", "endpoints", "configmaps", "secrets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"extensions"},
					Resources: []string{"daemonsets", "deployments", "replicasets", "ingresses"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"statefulsets", "deployments"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"cronjobs", "jobs"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"autoscaling"},
					Resources: []string{"horizontalpodautoscalers"},
					Verbs:     []string{"list", "watch"},
				},
				// for Prometheus-Auth Agent
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces", "serviceaccounts", "secrets"},
					Verbs:     []string{"list", "watch", "get"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"roles", "clusterroles", "rolebindings", "clusterrolebindings"},
					Verbs:     []string{"list", "watch", "get"},
				},
			}

			if appClusterRole.Name == appClusterRoleName {
				if appClusterRole.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ClusterRole is still on terminating", appClusterRoleName))
				}

				// ensure
				appClusterRole = appClusterRole.DeepCopy()
				appClusterRole.Rules = rules
				if _, err := ah.agentRBACClient.ClusterRoles(metav1.NamespaceAll).Update(appClusterRole); err != nil {
					return errors.Wrapf(err, "failed to update %q ClusterRole", appClusterRoleName)
				}
			} else {
				appClusterRole = &k8srbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:   appClusterRoleName,
						Labels: ownedLabels,
					},
					Rules: rules,
				}

				if _, err := ah.agentRBACClient.ClusterRoles(metav1.NamespaceAll).Create(appClusterRole); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ClusterRole", appClusterRoleName)
				}
			}

			return nil
		},

		// detect ClusterRoleBinding (the name is ${ServiceAccountName}-binding)
		func() error {
			appClusterRoleBinding, err := ah.agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Get(appClusterRoleBindingName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to query %q ClusterRoleBinding", appClusterRoleBindingName)
			}
			if appClusterRoleBinding.Name == appClusterRoleBindingName {
				if appClusterRoleBinding.DeletionTimestamp != nil {
					return errors.New(fmt.Sprintf("stale %q ClusterRoleBinding is still on terminating", appClusterRoleBindingName))
				}
			} else {
				appClusterRoleBinding = &k8srbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   appClusterRoleBindingName,
						Labels: ownedLabels,
					},
					Subjects: []k8srbacv1.Subject{
						{
							Kind:      k8srbacv1.ServiceAccountKind,
							Namespace: appTargetNamespace,
							Name:      appServiceAccountName,
						},
					},
					RoleRef: k8srbacv1.RoleRef{
						APIGroup: k8srbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     appClusterRoleName,
					},
				}

				if _, err := ah.agentRBACClient.ClusterRoleBindings(metav1.NamespaceAll).Create(appClusterRoleBinding); err != nil && !k8serrors.IsAlreadyExists(err) {
					return errors.Wrapf(err, "failed to create %q ClusterRoleBinding", appClusterRoleBindingName)
				}
			}

			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return appServiceAccountName, nil
}

func (ah *appHandler) deployClusterMonitoringApp(appName, appTargetNamespace string, appServiceAccountName, appProjectName string, cluster *mgmtv3.Cluster, etcdTLSConfig []*etcdTLSConfig, systemComponentMap map[string][]string) error {
	alertSvcName, _, alertPort := monitoring.ClusterAlertManagerEndpoint()

	appAnswers := map[string]string{
		"exporter-coredns.enabled":  "true",
		"exporter-coredns.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-controller-manager.enabled":  "true",
		"exporter-kube-controller-manager.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-dns.enabled":  "true",
		"exporter-kube-dns.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-etcd.enabled":  "false",
		"exporter-kube-etcd.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-scheduler.enabled":  "true",
		"exporter-kube-scheduler.apiGroup": monitoring.APIVersion.Group,

		"exporter-kube-state.enabled":            "true",
		"exporter-kube-state.apiGroup":           monitoring.APIVersion.Group,
		"exporter-kube-state.serviceAccountName": appServiceAccountName,

		"exporter-kubelets.enabled":  "true",
		"exporter-kubelets.apiGroup": monitoring.APIVersion.Group,

		"exporter-kubernetes.enabled":  "true",
		"exporter-kubernetes.apiGroup": monitoring.APIVersion.Group,

		"exporter-node.enabled":            "true",
		"exporter-node.apiGroup":           monitoring.APIVersion.Group,
		"exporter-node.serviceAccountName": appServiceAccountName,

		"exporter-fluentd.enabled":  "true",
		"exporter-fluentd.apiGroup": monitoring.APIVersion.Group,

		"grafana.enabled":             "true",
		"grafana.apiGroup":            monitoring.APIVersion.Group,
		"grafana.serviceAccountName":  appServiceAccountName,
		"grafana.persistence.enabled": "false",

		"prometheus.enabled":                                                      "true",
		"prometheus.apiGroup":                                                     monitoring.APIVersion.Group,
		"prometheus.serviceAccountName":                                           appServiceAccountName,
		"prometheus.persistence.enabled":                                          "false",
		"prometheus.alertingEndpoints[0].name":                                    alertSvcName,
		"prometheus.alertingEndpoints[0].namespace":                               appTargetNamespace,
		"prometheus.alertingEndpoints[0].port":                                    alertPort,
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].key":      nslabels.ProjectIDFieldLabel,
		"prometheus.serviceMonitorNamespaceSelector.matchExpressions[0].operator": "Exists",
		"prometheus.serviceMonitorSelector.matchExpressions[0].key":               monitoring.CattlePrometheusRuleLabelKey,
		"prometheus.serviceMonitorSelector.matchExpressions[0].operator":          "In",
		"prometheus.serviceMonitorSelector.matchExpressions[0].values[1]":         "rancher-monitoring",
		"prometheus.ruleNamespaceSelector.matchExpressions[0].key":                nslabels.ProjectIDFieldLabel,
		"prometheus.ruleNamespaceSelector.matchExpressions[0].operator":           "Exists",
		"prometheus.rulesSelector.matchExpressions[0].key":                        monitoring.CattlePrometheusRuleLabelKey,
		"prometheus.rulesSelector.matchExpressions[0].operator":                   "In",
		"prometheus.rulesSelector.matchExpressions[0].values[0]":                  monitoring.CattleAlertingPrometheusRuleLabelValue,
		"prometheus.rulesSelector.matchExpressions[0].values[1]":                  "rancher-monitoring",
	}
	appAnswers = monitoring.OverwriteAppAnswers(appAnswers, cluster.Annotations)

	if systemComponentMap != nil {
		if etcdEndpoints, ok := systemComponentMap[etcd]; ok {
			appAnswers["prometheus.secrets[0]"] = exporterEtcdCertName
			appAnswers["exporter-kube-etcd.enabled"] = "true"
			appAnswers["exporter-kube-etcd.ports.metrics.port"] = "2379"
			for k, v := range etcdEndpoints {
				key := fmt.Sprintf("exporter-kube-etcd.endpoints[%d]", k)
				appAnswers[key] = v
			}

			if etcdTLSConfig != nil {
				appAnswers["exporter-kube-etcd.certFile"] = etcdTLSConfig[0].certPath
				appAnswers["exporter-kube-etcd.keyFile"] = etcdTLSConfig[0].keyPath
			}
		}

		if controlplaneEndpoints, ok := systemComponentMap[controlplane]; ok {
			appAnswers["exporter-kube-scheduler.enabled"] = "true"
			appAnswers["exporter-kube-controller-manager.enable"] = "true"
			for k, v := range controlplaneEndpoints {
				key1 := fmt.Sprintf("exporter-kube-scheduler.endpoints[%d]", k)
				key2 := fmt.Sprintf("exporter-kube-controller-manager.endpoints[%d]", k)
				appAnswers[key1] = v
				appAnswers[key2] = v
			}
		}
	}

	appCatalogID := settings.SystemMonitoringCatalogID.Get()
	_, appDeployProjectID := ref.Parse(appProjectName)
	app := &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: monitoring.CopyCreatorID(nil, cluster.Annotations),
			Labels:      monitoring.OwnedLabels(appName, appTargetNamespace, monitoring.ClusterLevel),
			Name:        appName,
			Namespace:   appDeployProjectID,
		},
		Spec: projectv3.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher Cluster Monitoring",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: appTargetNamespace,
		},
	}

	err := monitoring.DeployApp(ah.cattleAppsGetter, appDeployProjectID, app)
	if err != nil {
		return err
	}

	return nil
}

func (ah *appHandler) revokePermissions(appServiceAccountName, appClusterRoleName, appClusterRoleBindingName, appTargetNamespace string) error {
	return monitoring.RevokePermissions(ah.agentRBACClient, ah.agentCoreClient, appServiceAccountName, appClusterRoleName, appClusterRoleBindingName, appTargetNamespace)
}

func (ah *appHandler) withdrawApp(appName, appTargetNamespace string) error {
	return monitoring.WithdrawApp(ah.cattleAppsGetter, monitoring.OwnedAppListOptions(appName, appTargetNamespace))
}

func getClusterTag(cluster *mgmtv3.Cluster) string {
	return fmt.Sprintf("%s(%s)", cluster.Spec.DisplayName, cluster.Name)
}

func isRkeCluster(cluster *mgmtv3.Cluster) bool {
	return cluster.Status.Driver == mgmtv3.ClusterDriverRKE
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
