package deployer

import (
	"fmt"
	"reflect"
	"time"

	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/types/slice"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	manager2 "github.com/rancher/rancher/pkg/catalog/manager"
	alertutil "github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert/manager"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/namespace"
	projectutil "github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	creatorIDAnn          = "field.cattle.io/creatorId"
	systemProjectLabel    = map[string]string{"authz.management.cattle.io/system-project": "true"}
	WebhookReceiverEnable = "webhook-receiver.enabled"

	webhookReceiverTypes = []string{
		"dingtalk",
		"msteams",
	}
)

type Deployer struct {
	clusterName             string
	alertManager            *manager.AlertManager
	clusterAlertGroupLister mgmtv3.ClusterAlertGroupLister
	projectAlertGroupLister mgmtv3.ProjectAlertGroupLister
	notifierLister          mgmtv3.NotifierLister
	projectLister           mgmtv3.ProjectLister
	clusters                mgmtv3.ClusterInterface
	clusterLister           mgmtv3.ClusterLister
	appDeployer             *appDeployer
}

type appDeployer struct {
	appsGetter           projectv3.AppsGetter
	appsLister           projectv3.AppLister
	catalogManager       manager2.CatalogManager
	namespaces           v1.NamespaceInterface
	secrets              v1.SecretInterface
	templateLister       mgmtv3.CatalogTemplateLister
	statefulsets         appsv1.StatefulSetInterface
	systemAccountManager *systemaccount.Manager
	deployments          appsv1.DeploymentInterface
}

func NewDeployer(cluster *config.UserContext, manager *manager.AlertManager) *Deployer {
	appsgetter := cluster.Management.Project
	ad := &appDeployer{
		appsGetter:           appsgetter,
		appsLister:           cluster.Management.Project.Apps("").Controller().Lister(),
		catalogManager:       cluster.Management.CatalogManager,
		namespaces:           cluster.Core.Namespaces(metav1.NamespaceAll),
		secrets:              cluster.Core.Secrets(metav1.NamespaceAll),
		templateLister:       cluster.Management.Management.CatalogTemplates(namespace.GlobalNamespace).Controller().Lister(),
		statefulsets:         cluster.Apps.StatefulSets(metav1.NamespaceAll),
		systemAccountManager: systemaccount.NewManager(cluster.Management),
		deployments:          cluster.Apps.Deployments(metav1.NamespaceAll),
	}

	return &Deployer{
		clusterName:             cluster.ClusterName,
		alertManager:            manager,
		clusterAlertGroupLister: cluster.Management.Management.ClusterAlertGroups(cluster.ClusterName).Controller().Lister(),
		projectAlertGroupLister: cluster.Management.Management.ProjectAlertGroups(metav1.NamespaceAll).Controller().Lister(),
		notifierLister:          cluster.Management.Management.Notifiers(cluster.ClusterName).Controller().Lister(),
		projectLister:           cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		clusters:                cluster.Management.Management.Clusters(metav1.NamespaceAll),
		clusterLister:           cluster.Management.Management.Clusters(metav1.NamespaceAll).Controller().Lister(),
		appDeployer:             ad,
	}
}

func (d *Deployer) ProjectGroupSync(key string, alert *mgmtv3.ProjectAlertGroup) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *Deployer) ClusterGroupSync(key string, alert *mgmtv3.ClusterAlertGroup) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *Deployer) ProjectRuleSync(key string, alert *mgmtv3.ProjectAlertRule) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *Deployer) ClusterRuleSync(key string, alert *mgmtv3.ClusterAlertRule) (runtime.Object, error) {
	return nil, d.sync()
}

// //deploy or clean up resources(alertmanager deployment, service, namespace) required by alerting.
func (d *Deployer) sync() error {
	appName, appTargetNamespace := monitorutil.ClusterAlertManagerInfo()

	systemProject, err := projectutil.GetSystemProject(d.clusterName, d.projectLister)
	if err != nil {
		return err
	}

	systemProjectID := ref.Ref(systemProject)

	needDeploy, needWebhookReceiver, err := d.needDeploy()
	if err != nil {
		return fmt.Errorf("check alertmanager deployment failed, %v", err)
	}

	cluster, err := d.clusterLister.Get("", d.clusterName)
	if err != nil {
		return fmt.Errorf("get cluster %s failed, %v", d.clusterName, err)
	}
	newCluster := cluster.DeepCopy()
	newCluster.Spec.EnableClusterAlerting = needDeploy

	if needDeploy {
		operatorAppName, operatorAppNamespace := monitorutil.SystemMonitoringInfo()
		operatorWorkload, err := d.appDeployer.deployments.GetNamespaced(operatorAppNamespace, fmt.Sprintf("prometheus-operator-%s", operatorAppName), metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("get deployment %s/prometheus-operator-%s failed, %v", operatorAppNamespace, operatorAppName, err)
		}
		if operatorWorkload == nil || operatorWorkload.DeletionTimestamp != nil {
			d.clusters.Controller().Enqueue(metav1.NamespaceAll, d.clusterName)
		}

		if !reflect.DeepEqual(cluster, newCluster) {
			cluster, err := d.clusters.Update(newCluster)
			if err != nil {
				return fmt.Errorf("update cluster %v failed, %v", d.clusterName, err)
			}

			newCluster = cluster.DeepCopy()
		}

		if d.alertManager.IsDeploy, err = d.appDeployer.deploy(appName, appTargetNamespace, systemProjectID, needWebhookReceiver, d.clusterLister, d.clusterName); err != nil {
			return fmt.Errorf("deploy alertmanager failed, %v", err)
		}

		if err = d.appDeployer.isDeploySuccess(newCluster, alertutil.GetAlertManagerDaemonsetName(appName), appTargetNamespace); err != nil {
			return err
		}
	} else {
		if d.alertManager.IsDeploy, err = d.appDeployer.cleanup(appName, appTargetNamespace, systemProjectID); err != nil {
			return fmt.Errorf("clean up alertmanager failed, %v", err)
		}
		v32.ClusterConditionAlertingEnabled.False(newCluster)
	}

	if !reflect.DeepEqual(cluster, newCluster) {
		_, err = d.clusters.Update(newCluster)
		if err != nil {
			return fmt.Errorf("update cluster %v failed, %v", d.clusterName, err)
		}
	}

	return nil

}

// //only deploy the alertmanager when notifier is configured and alert is using it.
func (d *Deployer) needDeploy() (bool, bool, error) {
	needDeploy := false
	needWebhookReceiver := false

	notifiers, err := d.notifierLister.List("", labels.NewSelector())
	if err != nil {
		return false, false, err
	}

	if len(notifiers) == 0 {
		return false, false, err
	}

	clusterAlerts, err := d.clusterAlertGroupLister.List("", labels.NewSelector())
	if err != nil {
		return false, false, err
	}

	for _, alert := range clusterAlerts {
		if len(alert.Spec.Recipients) > 0 {
			needDeploy = true
			for _, r := range alert.Spec.Recipients {
				if slice.ContainsString(webhookReceiverTypes, r.NotifierType) {
					needWebhookReceiver = true
					return needDeploy, needWebhookReceiver, nil
				}
			}
		}
	}

	projectAlerts, err := d.projectAlertGroupLister.List("", labels.NewSelector())
	if err != nil {
		return false, false, nil
	}

	for _, alert := range projectAlerts {
		if controller.ObjectInCluster(d.clusterName, alert) {
			if len(alert.Spec.Recipients) > 0 {
				needDeploy = true
				for _, r := range alert.Spec.Recipients {
					if slice.ContainsString(webhookReceiverTypes, r.NotifierType) {
						needWebhookReceiver = true
						return needDeploy, needWebhookReceiver, nil
					}
				}
			}
		}
	}

	return needDeploy, needWebhookReceiver, nil
}

func (d *appDeployer) isDeploySuccess(cluster *mgmtv3.Cluster, appName, appTargetNamespace string) error {
	_, err := v32.ClusterConditionAlertingEnabled.DoUntilTrue(cluster, func() (runtime.Object, error) {
		_, err := d.statefulsets.GetNamespaced(appTargetNamespace, appName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				time.Sleep(5 * time.Second)
			}
			return nil, fmt.Errorf("failed to get Alertmanager Deployment information, %v", err)
		}
		return cluster, nil
	})

	return err
}

func (d *appDeployer) cleanup(appName, appTargetNamespace, systemProjectID string) (bool, error) {
	_, systemProjectName := ref.Parse(systemProjectID)

	var errgrp errgroup.Group

	errgrp.Go(func() error {
		if _, err := d.appsLister.Get(systemProjectName, appName); err != nil {
			if apierrors.IsNotFound(err) {
				// the app doesn't exist
				return nil
			}
			return err
		}
		return d.appsGetter.Apps(systemProjectName).Delete(appName, &metav1.DeleteOptions{})
	})

	errgrp.Go(func() error {
		secretName := alertutil.GetAlertManagerSecretName(appName)
		return d.secrets.DeleteNamespaced(appTargetNamespace, secretName, &metav1.DeleteOptions{})
	})

	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}

	return false, nil
}

func (d *appDeployer) getSecret(secretName, secretNamespace string) *corev1.Secret {
	cfg := manager.GetAlertManagerDefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      secretName,
		},
		Data: map[string][]byte{
			"alertmanager.yaml": data,
			"notification.tmpl": []byte(NotificationTmpl),
		},
	}
}

func (d *appDeployer) deploy(appName, appTargetNamespace, systemProjectID string, needWebhookReceiver bool, clusterLister mgmtv3.ClusterLister, clusterName string) (bool, error) {
	clusterName, systemProjectName := ref.Parse(systemProjectID)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: appTargetNamespace,
		},
	}

	if _, err := d.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return false, fmt.Errorf("create ns %s failed, %v", appTargetNamespace, err)
	}

	secretName := alertutil.GetAlertManagerSecretName(appName)
	secret := d.getSecret(secretName, appTargetNamespace)
	if _, err := d.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return false, fmt.Errorf("create secret %s:%s failed, %v", appTargetNamespace, appName, err)
	}

	app, err := d.appsLister.Get(systemProjectName, appName)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to query %q App in %s Project, %v", appName, systemProjectName, err)
	}

	enableWebhookReceiver := fmt.Sprintf("%t", needWebhookReceiver)
	if app != nil && app.Name == appName {
		if app.DeletionTimestamp != nil {
			return false, fmt.Errorf("stale %q App in %s Project is still on terminating", appName, systemProjectName)
		}

		if app.Spec.Answers[WebhookReceiverEnable] != enableWebhookReceiver {
			copy := app.DeepCopy()
			copy.Spec.Answers[WebhookReceiverEnable] = enableWebhookReceiver
			_, err := d.appsGetter.Apps(systemProjectName).Update(copy)
			if err != nil {
				return false, fmt.Errorf("failed to update %q App, %v", appName, err)
			}
		}
		return true, nil
	}

	template, err := d.templateLister.Get(namespace.GlobalNamespace, monitorutil.RancherMonitoringTemplateName)
	if err != nil {
		return false, fmt.Errorf("get template %s:%s failed, %v", namespace.GlobalNamespace, monitorutil.RancherMonitoringTemplateName, err)
	}
	templateVersion, err := d.catalogManager.LatestAvailableTemplateVersion(template, clusterName)
	if err != nil {
		return false, err
	}

	creator, err := d.systemAccountManager.GetSystemUser(clusterName)
	if err != nil {
		return false, err
	}

	app = &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: creator.Name,
			},
			Labels:    monitorutil.OwnedLabels(appName, appTargetNamespace, systemProjectID, monitorutil.SystemLevel),
			Name:      appName,
			Namespace: systemProjectName,
		},
		Spec: v33.AppSpec{
			Answers: map[string]string{
				"alertmanager.enabled":                "true",
				"alertmanager.serviceMonitor.enabled": "true",
				"alertmanager.apiGroup":               monitorutil.APIVersion.Group,
				"alertmanager.enabledRBAC":            "false",
				"alertmanager.configFromSecret":       secret.Name,
				"operator.enabled":                    "false",
				WebhookReceiverEnable:                 enableWebhookReceiver,
			},
			Description:     "Alertmanager for Rancher Monitoring",
			ExternalID:      templateVersion.ExternalID,
			ProjectName:     systemProjectID,
			TargetNamespace: appTargetNamespace,
		},
	}
	if _, err := d.appsGetter.Apps(systemProjectName).Create(app); err != nil && !apierrors.IsAlreadyExists(err) {
		return false, fmt.Errorf("failed to create %q App, %v", appName, err)
	}

	return true, nil
}
