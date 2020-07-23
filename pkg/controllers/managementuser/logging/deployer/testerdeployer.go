package deployer

import (
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/managementuser/logging/config"
	"github.com/rancher/rancher/pkg/controllers/managementuser/logging/generator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"

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
	systemAccountManager *systemaccount.Manager
}

func NewTesterDeployer(mgmtCtx *config.ManagementContext, appsGetter projectv3.AppsGetter, appLister projectv3.AppLister, projectLister mgmtv3.ProjectLister, podLister v1.PodLister, projectLoggingLister mgmtv3.ProjectLoggingLister, namespaces v1.NamespaceInterface, templateLister mgmtv3.CatalogTemplateLister) *TesterDeployer {
	appDeployer := &AppDeployer{
		AppsGetter: appsGetter,
		AppsLister: appLister,
		Namespaces: namespaces,
		PodLister:  podLister,
	}

	return &TesterDeployer{
		projectLister:        projectLister,
		templateLister:       templateLister,
		projectLoggingLister: projectLoggingLister,
		namespacesLister:     namespaces.Controller().Lister(),
		appDeployer:          appDeployer,
		systemAccountManager: systemaccount.NewManager(mgmtCtx),
	}
}

func (d *TesterDeployer) Deploy(level, clusterName, projectID string, loggingTarget v32.LoggingTargets) error {
	systemProject, err := project.GetSystemProject(clusterName, d.projectLister)
	if err != nil {
		return err
	}

	// This app will be deployed to system project, and the chart has clusterrole rbac, project user couldn't create clusterroles rbac, so use cluster system user.
	creator, err := d.systemAccountManager.GetSystemUser(systemProject.Spec.ClusterName)
	if err != nil {
		return err
	}

	systemProjectID := ref.Ref(systemProject)
	if err = d.deployLoggingTester(systemProjectID, creator.Name, level, clusterName, projectID, loggingTarget); err != nil {
		return err
	}

	return d.isLoggingTesterDeploySuccess()
}

func (d *TesterDeployer) deployLoggingTester(systemProjectID, appCreator, level, clusterName, projectID string, loggingTarget v32.LoggingTargets) error {
	templateVersionID := loggingconfig.RancherLoggingTemplateID()
	template, err := d.templateLister.Get(namespace.GlobalNamespace, templateVersionID)
	if err != nil {
		return errors.Wrapf(err, "failed to find template by ID %s", templateVersionID)
	}

	catalogID := loggingconfig.RancherLoggingCatalogID(template.Spec.DefaultVersion)

	var clusterSecret, projectSecret string
	if level == loggingconfig.ClusterLevel {
		spec := v32.ClusterLoggingSpec{
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
			Spec: v32.ProjectLoggingSpec{
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

	app := loggingTesterApp(appCreator, systemProjectID, catalogID, clusterSecret, projectSecret)

	return d.appDeployer.deploy(app, nil)
}

func (d *TesterDeployer) isLoggingTesterDeploySuccess() error {
	namespace := loggingconfig.LoggingNamespace
	return d.appDeployer.isDeploySuccess(namespace, loggingconfig.FluentdTesterSelector)
}
