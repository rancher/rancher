package deployer

import (
	"github.com/rancher/norman/controller"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"golang.org/x/sync/errgroup"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Deployer struct {
	clusterName          string
	clusterLister        mgmtv3.ClusterLister
	clusterLoggingLister mgmtv3.ClusterLoggingLister
	projectLoggingLister mgmtv3.ProjectLoggingLister
	projectLister        mgmtv3.ProjectLister
	templateLister       mgmtv3.CatalogTemplateLister
	appDeployer          *AppDeployer
}

func NewDeployer(cluster *config.UserContext, secretSyncer *configsyncer.SecretManager) *Deployer {
	clusterName := cluster.ClusterName
	appsgetter := cluster.Management.Project

	appDeployer := &AppDeployer{
		AppsGetter: appsgetter,
		Namespaces: cluster.Core.Namespaces(metav1.NamespaceAll),
		Pods:       cluster.Core.Pods(metav1.NamespaceAll),
	}

	return &Deployer{
		clusterName:          clusterName,
		clusterLister:        cluster.Management.Management.Clusters(metav1.NamespaceAll).Controller().Lister(),
		clusterLoggingLister: cluster.Management.Management.ClusterLoggings(clusterName).Controller().Lister(),
		projectLoggingLister: cluster.Management.Management.ProjectLoggings(metav1.NamespaceAll).Controller().Lister(),
		projectLister:        cluster.Management.Management.Projects(metav1.NamespaceAll).Controller().Lister(),
		templateLister:       cluster.Management.Management.CatalogTemplates(metav1.NamespaceAll).Controller().Lister(),
		appDeployer:          appDeployer,
	}
}

func (d *Deployer) ClusterLoggingSync(key string, obj *mgmtv3.ClusterLogging) (runtime.Object, error) {
	return obj, d.sync()
}

func (d *Deployer) ProjectLoggingSync(key string, obj *mgmtv3.ProjectLogging) (runtime.Object, error) {
	return obj, d.sync()
}

func (d *Deployer) sync() error {
	appName := loggingconfig.AppName
	namepspace := loggingconfig.LoggingNamespace

	systemProject, err := project.GetSystemProject(d.clusterName, d.projectLister)
	if err != nil {
		return err
	}

	systemProjectCreator := systemProject.Annotations[creatorIDAnn]
	systemProjectID := ref.Ref(systemProject)

	allDisabled, err := d.isAllLoggingDisable()
	if err != nil {
		return err
	}

	if allDisabled {
		return d.appDeployer.cleanup(appName, namepspace, systemProjectID)
	}

	return d.deploy(systemProjectID, systemProjectCreator)
}

func (d *Deployer) deploy(systemProjectID, systemProjectCreator string) error {
	if err := d.deployRancherLogging(systemProjectID, systemProjectCreator); err != nil {
		return err
	}

	return d.isRancherLoggingDeploySuccess()
}

func (d *Deployer) deployRancherLogging(systemProjectID, systemProjectCreator string) error {
	cluster, err := d.clusterLister.Get("", d.clusterName)
	if err != nil {
		return errors.Wrapf(err, "get dockerRootDir from cluster %s failed", d.clusterName)
	}

	driverDir := getDriverDir(cluster.Status.Driver)

	templateVersionID := loggingconfig.RancherLoggingTemplateID()
	template, err := d.templateLister.Get(namespace.GlobalNamespace, templateVersionID)
	if err != nil {
		return errors.Wrapf(err, "failed to find template by ID %s", templateVersionID)
	}

	catalogID := loggingconfig.RancherLoggingCatalogID(template.Spec.DefaultVersion)

	app := rancherLoggingApp(systemProjectCreator, systemProjectID, catalogID, driverDir)

	return d.appDeployer.deploy(app)
}

func (d *Deployer) isRancherLoggingDeploySuccess() error {
	namespace := loggingconfig.LoggingNamespace

	var errgrp errgroup.Group

	errgrp.Go(func() error {
		return d.appDeployer.isDeploySuccess(namespace, loggingconfig.FluentdSelector)
	})

	errgrp.Go(func() error {
		return d.appDeployer.isDeploySuccess(namespace, loggingconfig.LogAggregatorSelector)
	})

	return errgrp.Wait()
}

func (d *Deployer) isAllLoggingDisable() (bool, error) {
	clusterLoggings, err := d.clusterLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	allClusterProjectLoggings, err := d.projectLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	var projectLoggings []*mgmtv3.ProjectLogging
	for _, v := range allClusterProjectLoggings {
		if controller.ObjectInCluster(d.clusterName, v) {
			projectLoggings = append(projectLoggings, v)
		}
	}

	if len(clusterLoggings) == 0 && len(projectLoggings) == 0 {
		return true, nil
	}

	for _, v := range clusterLoggings {
		wl := utils.NewLoggingTargetTestWrap(v.Spec.LoggingTargets)
		if wl != nil {
			return false, nil
		}
	}

	for _, v := range projectLoggings {
		wpl := utils.NewLoggingTargetTestWrap(v.Spec.LoggingTargets)
		if wpl != nil {
			return false, nil
		}
	}
	return true, nil
}

func getDriverDir(driverName string) string {
	switch driverName {
	case mgmtv3.ClusterDriverRKE:
		return "/var/lib/kubelet/volumeplugins"
	case loggingconfig.GoogleKubernetesEngine:
		return "/home/kubernetes/flexvolume"
	default:
		return "/usr/libexec/kubernetes/kubelet-plugins/volume/exec"
	}
}
