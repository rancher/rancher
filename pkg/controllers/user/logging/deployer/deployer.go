package deployer

import (
	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	versionutil "github.com/rancher/rancher/pkg/catalog/utils"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	v1 "github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	fluentdSystemWriteKeys = []string{
		"fluentd.cluster.dockerRoot",
		"fluentd.fluentd-linux.cluster.dockerRoot",
		"fluentd.fluentd-windows.enabled",
	}
	windowNodeLabel = labels.Set(map[string]string{"beta.kubernetes.io/os": "windows"}).AsSelector()
)

type Deployer struct {
	clusterName          string
	clusterLister        mgmtv3.ClusterLister
	clusterLoggingLister mgmtv3.ClusterLoggingLister
	nodeLister           v1.NodeLister
	projectLoggingLister mgmtv3.ProjectLoggingLister
	projectLister        mgmtv3.ProjectLister
	templateLister       mgmtv3.CatalogTemplateLister
	appDeployer          *AppDeployer
	systemAccountManager *systemaccount.Manager
}

func NewDeployer(cluster *config.UserContext, secretSyncer *configsyncer.SecretManager) *Deployer {
	clusterName := cluster.ClusterName
	appsgetter := cluster.Management.Project

	appDeployer := &AppDeployer{
		AppsGetter: appsgetter,
		AppsLister: cluster.Management.Project.Apps("").Controller().Lister(),
		Namespaces: cluster.Core.Namespaces(metav1.NamespaceAll),
		PodLister:  cluster.Core.Pods(metav1.NamespaceAll).Controller().Lister(),
	}

	return &Deployer{
		clusterName:          clusterName,
		clusterLister:        cluster.Management.Management.Clusters(metav1.NamespaceAll).Controller().Lister(),
		clusterLoggingLister: cluster.Management.Management.ClusterLoggings(clusterName).Controller().Lister(),
		nodeLister:           cluster.Core.Nodes(metav1.NamespaceAll).Controller().Lister(),
		projectLoggingLister: cluster.Management.Management.ProjectLoggings(metav1.NamespaceAll).Controller().Lister(),
		projectLister:        cluster.Management.Management.Projects(metav1.NamespaceAll).Controller().Lister(),
		templateLister:       cluster.Management.Management.CatalogTemplates(metav1.NamespaceAll).Controller().Lister(),
		appDeployer:          appDeployer,
		systemAccountManager: systemaccount.NewManager(cluster.Management),
	}
}

func (d *Deployer) ClusterLoggingSync(key string, obj *mgmtv3.ClusterLogging) (runtime.Object, error) {
	return obj, d.sync()
}

func (d *Deployer) ProjectLoggingSync(key string, obj *mgmtv3.ProjectLogging) (runtime.Object, error) {
	return obj, d.sync()
}

func (d *Deployer) ClusterSync(key string, obj *mgmtv3.Cluster) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Name != d.clusterName {
		return obj, nil
	}
	return obj, d.sync()
}

func (d *Deployer) NodeSync(key string, obj *corev1.Node) (runtime.Object, error) {
	return obj, d.sync()
}

func (d *Deployer) sync() error {
	appName := loggingconfig.AppName
	namepspace := loggingconfig.LoggingNamespace

	systemProject, err := project.GetSystemProject(d.clusterName, d.projectLister)
	if err != nil {
		return err
	}

	systemProjectID := ref.Ref(systemProject)

	allDisabled, err := d.isAllLoggingDisable()
	if err != nil {
		return err
	}

	if allDisabled {
		return d.appDeployer.cleanup(appName, namepspace, systemProjectID)
	}

	creator, err := d.systemAccountManager.GetSystemUser(systemProject.Spec.ClusterName)
	if err != nil {
		return err
	}

	return d.deploy(systemProjectID, creator.Name)
}

func (d *Deployer) deploy(systemProjectID, appCreator string) error {
	if err := d.deployRancherLogging(systemProjectID, appCreator); err != nil {
		return err
	}

	return d.isRancherLoggingDeploySuccess()
}

func (d *Deployer) deployRancherLogging(systemProjectID, appCreator string) error {
	cluster, err := d.clusterLister.Get("", d.clusterName)
	if err != nil {
		return errors.Wrapf(err, "get dockerRootDir from cluster %s failed", d.clusterName)
	}

	dockerRootDir := cluster.Spec.DockerRootDir
	if dockerRootDir == "" {
		dockerRootDir = settings.InitialDockerRootDir.Get()
	}

	driverDir := getDriverDir(cluster.Status.Driver)

	templateVersionID := loggingconfig.RancherLoggingTemplateID()
	template, err := d.templateLister.Get(namespace.GlobalNamespace, templateVersionID)
	if err != nil {
		return errors.Wrapf(err, "failed to find template by ID %s", templateVersionID)
	}

	templateVersion, err := versionutil.LatestAvailableTemplateVersion(template)
	if err != nil {
		return err
	}

	var clusterLogging *mgmtv3.ClusterLogging
	if cls, err := d.clusterLoggingLister.List("", labels.NewSelector()); err == nil {
		for _, cl := range cls {
			if cl.Spec.ClusterName == cluster.Name {
				clusterLogging = cl
			}
		}
	} else {
		return err
	}

	var tolerations []corev1.Toleration
	if clusterLogging != nil {
		tolerations = clusterLogging.Spec.Tolerations
	} else {
		tolerations = []corev1.Toleration{}
	}

	app := rancherLoggingApp(appCreator, systemProjectID, templateVersion.ExternalID, driverDir, dockerRootDir, tolerations)

	windowsNodes, err := d.nodeLister.List(metav1.NamespaceAll, windowNodeLabel)
	if err != nil {
		return errors.Wrapf(err, "failed to list nodes")
	}

	updateInRancherLoggingAppWindowsConfig(app, len(windowsNodes) != 0)

	return d.appDeployer.deploy(app, fluentdSystemWriteKeys)
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
