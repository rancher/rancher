package deployer

import (
	"fmt"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	creatorIDAnn       = "field.cattle.io/creatorId"
	systemProjectLabel = map[string]string{"authz.management.cattle.io/system-project": "true"}
)

type Deployer struct {
	clusterName             string
	alertManager            *manager.AlertManager
	namespaces              v1.NamespaceInterface
	secrets                 v1.SecretInterface
	serviceLister           v1.ServiceLister
	appsGetter              projectv3.AppsGetter
	clusterAlertGroupLister mgmtv3.ClusterAlertGroupLister
	projectAlertGroupLister mgmtv3.ProjectAlertGroupLister
	notifierLister          mgmtv3.NotifierLister
	projectLister           mgmtv3.ProjectLister
	templateVersions        mgmtv3.TemplateVersionInterface
}

func NewDeployer(cluster *config.UserContext, manager *manager.AlertManager) *Deployer {
	return &Deployer{
		clusterName:             cluster.ClusterName,
		alertManager:            manager,
		namespaces:              cluster.Core.Namespaces(metav1.NamespaceAll),
		secrets:                 cluster.Core.Secrets(metav1.NamespaceAll),
		serviceLister:           cluster.Core.Services(metav1.NamespaceAll).Controller().Lister(),
		appsGetter:              cluster.Management.Project,
		clusterAlertGroupLister: cluster.Management.Management.ClusterAlertGroups(cluster.ClusterName).Controller().Lister(),
		projectAlertGroupLister: cluster.Management.Management.ProjectAlertGroups(metav1.NamespaceAll).Controller().Lister(),
		notifierLister:          cluster.Management.Management.Notifiers(cluster.ClusterName).Controller().Lister(),
		projectLister:           cluster.Management.Management.Projects(metav1.NamespaceAll).Controller().Lister(),
		templateVersions:        cluster.Management.Management.TemplateVersions(metav1.NamespaceAll),
	}
}

func (d *Deployer) deploy(appName, appTargetNamespace, systemProjectID, systemProjectCreator string) error {
	_, systemProjectName := ref.Parse(systemProjectID)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: appTargetNamespace,
		},
	}
	if _, err := d.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create ns %s failed, %v", appTargetNamespace, err)
	}

	secretName := monitorutil.ClusterAlertManagerSecret()
	secret := d.getSecret(secretName)
	if _, err := d.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create secret %s:%s failed, %v", appTargetNamespace, appName, err)
	}

	app, err := d.appsGetter.Apps(systemProjectName).Get(appName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to query %q App in %s Project, %v", appName, systemProjectName, err)
	}
	if app.Name == appName {
		if app.DeletionTimestamp != nil {
			return fmt.Errorf("stale %q App in %s Project is still on terminating", appName, systemProjectName)
		}
		d.alertManager.IsDeploy = true
		return nil
	}

	// detect TemplateVersion "rancher-monitoring"
	catalogID := settings.SystemMonitoringCatalogID.Get()
	templateVersionID, err := common.ParseExternalID(catalogID)
	if err != nil {
		return fmt.Errorf("failed to parse catalog ID %q, %v", catalogID, err)
	}
	if _, err := d.templateVersions.Get(templateVersionID, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("failed to find catalog by ID %q, %v", catalogID, err)
	}

	// create App "metric expression"
	app = &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: systemProjectCreator,
			},
			Labels:    monitorutil.OwnedLabels(appName, appTargetNamespace, monitorutil.SystemLevel),
			Name:      appName,
			Namespace: systemProjectName,
		},
		Spec: projectv3.AppSpec{
			Answers: map[string]string{
				"alertmanager.enabled":                "true",
				"alertmanager.serviceMonitor.enabled": "true",
				"alertmanager.apiGroup":               monitorutil.APIVersion.Group,
				"alertmanager.enabledRBAC":            "false",
				"alertmanager.configFromSecret":       secretName,
			},
			Description:     "Alertmanager for Rancher Monitoring",
			ExternalID:      catalogID,
			ProjectName:     systemProjectID,
			TargetNamespace: appTargetNamespace,
		},
	}
	if _, err := d.appsGetter.Apps(systemProjectName).Create(app); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create %q App, %v", appName, err)
	}

	d.alertManager.IsDeploy = true
	return nil
}

func (d *Deployer) ProjectSync(key string, alert *mgmtv3.ProjectAlertGroup) (runtime.Object, error) {
	return nil, d.sync()
}

func (d *Deployer) ClusterSync(key string, alert *mgmtv3.ClusterAlertGroup) (runtime.Object, error) {
	return nil, d.sync()
}

// //deploy or clean up resources(alertmanager deployment, service, namespace) required by alerting.
func (d *Deployer) sync() error {
	appName, appTargetNamespace := monitorutil.ClusterAlertManagerInfo()

	defaultSystemProjects, err := d.projectLister.List(metav1.NamespaceAll, labels.Set(systemProjectLabel).AsSelector())
	if err != nil {
		return fmt.Errorf("list system project failed, %v", err)
	}

	if len(defaultSystemProjects) == 0 {
		return fmt.Errorf("get system project failed")
	}

	systemProject := defaultSystemProjects[0]
	systemProjectCreator := systemProject.Annotations[creatorIDAnn]
	systemProjectID := fmt.Sprintf("%s:%s", systemProject.Namespace, systemProject.Name)

	needDeploy, err := d.needDeploy()
	if err != nil {
		return fmt.Errorf("check alertmanager deployment failed, %v", err)
	}

	if needDeploy {
		return d.deploy(appName, appTargetNamespace, systemProjectID, systemProjectCreator)
	}

	return d.cleanup(appName, appTargetNamespace, systemProjectID)
}

// //only deploy the alertmanager when notifier is configured and alert is using it.
func (d *Deployer) needDeploy() (bool, error) {
	notifiers, err := d.notifierLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	if len(notifiers) == 0 {
		return false, err
	}

	clusterAlerts, err := d.clusterAlertGroupLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	for _, alert := range clusterAlerts {
		if len(alert.Spec.Recipients) > 0 {
			return true, nil
		}
	}

	projectAlerts, err := d.projectAlertGroupLister.List("", labels.NewSelector())
	if err != nil {
		return false, nil
	}

	for _, alert := range projectAlerts {
		if controller.ObjectInCluster(d.clusterName, alert) {
			if len(alert.Spec.Recipients) > 0 {
				return true, nil
			}
		}
	}

	return false, nil
}

func (d *Deployer) cleanup(appName, appTargetNamespace, systemProjectID string) error {
	_, systemProjectName := ref.Parse(systemProjectID)

	var errgrp errgroup.Group

	errgrp.Go(func() error {
		return d.appsGetter.Apps(systemProjectName).Delete(appName, &metav1.DeleteOptions{})
	})

	errgrp.Go(func() error {
		return d.secrets.DeleteNamespaced(appTargetNamespace, monitorutil.ClusterAlertManagerSecret(), &metav1.DeleteOptions{})
	})

	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	d.alertManager.IsDeploy = false
	return nil
}

func (d *Deployer) getSecret(secretName string) *corev1.Secret {
	cfg := d.alertManager.GetDefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: monitorutil.CattleNamespaceName,
			Name:      secretName,
		},
		Data: map[string][]byte{
			"alertmanager.yaml": data,
			"notification.tmpl": []byte(NotificationTmpl),
		},
	}
}
