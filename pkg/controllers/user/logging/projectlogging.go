package logging

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/labels"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
)

const (
	ProjectIDAnnotation = "field.cattle.io/projectId"
)

// ProjectLoggingSyncer listens for projectLogging CRD in management API
// and update the changes to configmap, deploy fluentd

type ProjectLoggingSyncer struct {
	projectLoggings      v3.ProjectLoggingInterface
	clusterLoggingLister v3.ClusterLoggingLister
	namespaces           v1.NamespaceInterface
	serviceAccounts      v1.ServiceAccountInterface
	configmaps           v1.ConfigMapInterface
	daemonsets           v1beta2.DaemonSetInterface
	clusterRoleBindings  rbacv1.ClusterRoleBindingInterface
	clusterLister        v3.ClusterLister
	clusterName          string
}

func registerProjectLogging(cluster *config.UserContext) {
	projectLoggings := cluster.Management.Management.ProjectLoggings("")
	syncer := &ProjectLoggingSyncer{
		projectLoggings:      projectLoggings,
		clusterLoggingLister: cluster.Management.Management.ClusterLoggings("").Controller().Lister(),
		serviceAccounts:      cluster.Core.ServiceAccounts(loggingconfig.LoggingNamespace),
		namespaces:           cluster.Core.Namespaces(""),
		configmaps:           cluster.Core.ConfigMaps(loggingconfig.LoggingNamespace),
		daemonsets:           cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace),
		clusterRoleBindings:  cluster.RBAC.ClusterRoleBindings(loggingconfig.LoggingNamespace),
		clusterLister:        cluster.Management.Management.Clusters("").Controller().Lister(),
		clusterName:          cluster.ClusterName,
	}
	projectLoggings.AddClusterScopedHandler("project-logging-controller", cluster.ClusterName, syncer.Sync)
}

func (c *ProjectLoggingSyncer) Sync(key string, obj *v3.ProjectLogging) error {
	//clean up
	if obj == nil || obj.DeletionTimestamp != nil {
		return utils.CleanResource(c.namespaces, c.clusterLoggingLister, c.projectLoggings.Controller().Lister())
	}

	if err := utils.IniteNamespace(c.namespaces); err != nil {
		return err
	}
	if err := utils.InitConfigMap(c.configmaps); err != nil {
		return err
	}
	if err := c.createOrUpdateProjectConfigMap(); err != nil {
		return err
	}

	if err := utils.CreateLogAggregator(c.daemonsets, c.serviceAccounts, c.clusterRoleBindings, c.clusterLister, c.clusterName, loggingconfig.LoggingNamespace); err != nil {
		return err
	}

	return utils.CreateFluentd(c.daemonsets, c.serviceAccounts, c.clusterRoleBindings, loggingconfig.LoggingNamespace)
}

func (c *ProjectLoggingSyncer) createOrUpdateProjectConfigMap() error {
	projectLoggings, err := c.projectLoggings.Controller().Lister().List("", labels.NewSelector())
	if err != nil {
		return errors.Wrap(err, "list project logging failed")
	}
	if len(projectLoggings) == 0 {
		return fmt.Errorf("no project logging configured")
	}
	ns, err := c.namespaces.Controller().Lister().List("", labels.NewSelector())
	if err != nil {
		return errors.Wrap(err, "list namespace failed")
	}
	var wl []utils.WrapProjectLogging
	for _, v := range projectLoggings {
		if controller.ObjectInCluster(c.clusterName, v) {
			var grepNamespace []string
			for _, v2 := range ns {
				if nsProjectName, ok := v2.Annotations[ProjectIDAnnotation]; ok && nsProjectName == v.Spec.ProjectName {
					grepNamespace = append(grepNamespace, v2.Name)
				}
			}

			formatgrepNamespace := fmt.Sprintf("(%s)", strings.Join(grepNamespace, "|"))
			projectLogging, err := utils.ToWrapProjectLogging(formatgrepNamespace, v.Spec)
			if err != nil {
				return err
			}
			wl = append(wl, *projectLogging)
		}
	}
	conf := make(map[string]interface{})
	conf["projectTargets"] = wl
	err = generator.GenerateConfigFile(loggingconfig.ProjectConfigPath, generator.ProjectTemplate, "project", conf)
	if err != nil {
		return errors.Wrap(err, "generate project config file failed")
	}
	return utils.UpdateConfigMap(loggingconfig.ProjectConfigPath, loggingconfig.ProjectLoggingName, "project", c.configmaps)
}
