package deployer

import (
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type TesterDeployer struct {
	projectLister        mgmtv3.ProjectLister
	templateLister       mgmtv3.CatalogTemplateLister
	projectLoggingLister mgmtv3.ProjectLoggingLister
	namespacesLister     v1.NamespaceLister
	appDeployer          *AppDeployer
}

func NewTesterDeployer(appsGetter projectv3.AppsGetter, projectLister mgmtv3.ProjectLister, pods v1.PodInterface, projectLoggingLister mgmtv3.ProjectLoggingLister, namespaces v1.NamespaceInterface, templateLister mgmtv3.CatalogTemplateLister) *TesterDeployer {
	appDeployer := &AppDeployer{
		AppsGetter: appsGetter,
		Namespaces: namespaces,
		Pods:       pods,
	}

	return &TesterDeployer{
		projectLister:        projectLister,
		templateLister:       templateLister,
		projectLoggingLister: projectLoggingLister,
		namespacesLister:     namespaces.Controller().Lister(),
		appDeployer:          appDeployer,
	}
}

func (d *TesterDeployer) Deploy(level, clusterName, projectID string, loggingTarget mgmtv3.LoggingTargets) error {
	systemProject, err := project.GetSystemProject(clusterName, d.projectLister)
	if err != nil {
		return err
	}

	systemProjectCreator := systemProject.Annotations[creatorIDAnn]
	systemProjectID := ref.Ref(systemProject)

	if err = d.deployLoggingTester(systemProjectID, systemProjectCreator, level, clusterName, projectID, loggingTarget); err != nil {
		return err
	}

	return d.isLoggingTesterDeploySuccess()
}

func (d *TesterDeployer) deployLoggingTester(systemProjectID, systemProjectCreator, level, clusterName, projectID string, loggingTarget mgmtv3.LoggingTargets) error {
	templateVersionID := loggingconfig.RancherLoggingTemplateID()
	template, err := d.templateLister.Get(namespace.GlobalNamespace, templateVersionID)
	if err != nil {
		return errors.Wrapf(err, "failed to find template by ID %s", templateVersionID)
	}

	catalogID := loggingconfig.RancherLoggingCatalogID(template.Spec.DefaultVersion)

	var clusterSecret, projectSecret string
	if level == loggingconfig.ClusterLevel {
		spec := mgmtv3.ClusterLoggingSpec{
			LoggingTargets: loggingTarget,
			ClusterName:    clusterName,
		}
		buf, err := generator.GenerateClusterConfig(spec, "", loggingconfig.DefaultCertDir)
		if err != nil {
			return err
		}

		clusterSecret = string(buf)

	} else if level == loggingconfig.ProjectLevel {

		cur, err := d.projectLoggingLister.List(metav1.NamespaceAll, labels.NewSelector())
		if err != nil {
			return errors.Wrap(err, "list project logging failed")
		}

		namespaces, err := d.namespacesLister.List(metav1.NamespaceAll, labels.NewSelector())
		if err != nil {
			return errors.New("list namespace failed")
		}

		new := &mgmtv3.ProjectLogging{
			Spec: mgmtv3.ProjectLoggingSpec{
				LoggingTargets: loggingTarget,
				ProjectName:    projectID,
			},
		}

		loggings := append(cur, new)

		buf, err := generator.GenerateProjectConfig(loggings, namespaces, systemProjectID, loggingconfig.DefaultCertDir)
		if err != nil {
			return err
		}
		projectSecret = string(buf)
	}

	app := loggingTesterApp(systemProjectCreator, systemProjectID, catalogID, clusterSecret, projectSecret)

	return d.appDeployer.deploy(app)
}

func (d *TesterDeployer) isLoggingTesterDeploySuccess() error {
	namespace := loggingconfig.LoggingNamespace
	return d.appDeployer.isDeploySuccess(namespace, loggingconfig.FluentdTesterSelector)
}
