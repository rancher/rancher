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
	"k8s.io/apimachinery/pkg/runtime"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
)

const (
	ProjectIDAnnotation = "field.cattle.io/projectId"
)

// ProjectLoggingLifecycle listens for projectLogging CRD in management API
// and update the changes to configmap, deploy fluentd

type ProjectLoggingLifecycle struct {
	clusterName          string
	clusterLister        v3.ClusterLister
	clusterRoleBindings  rbacv1.ClusterRoleBindingInterface
	clusterLoggingLister v3.ClusterLoggingLister
	configmaps           v1.ConfigMapInterface
	daemonsets           v1beta2.DaemonSetInterface
	namespaces           v1.NamespaceInterface
	projectLoggings      v3.ProjectLoggingInterface
	serviceAccounts      v1.ServiceAccountInterface
}

func registerProjectLogging(cluster *config.UserContext) {
	projectLoggings := cluster.Management.Management.ProjectLoggings("")
	lifecycle := &ProjectLoggingLifecycle{
		clusterName:          cluster.ClusterName,
		clusterLister:        cluster.Management.Management.Clusters("").Controller().Lister(),
		clusterRoleBindings:  cluster.RBAC.ClusterRoleBindings(loggingconfig.LoggingNamespace),
		clusterLoggingLister: cluster.Management.Management.ClusterLoggings("").Controller().Lister(),
		configmaps:           cluster.Core.ConfigMaps(loggingconfig.LoggingNamespace),
		daemonsets:           cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace),
		namespaces:           cluster.Core.Namespaces(""),
		projectLoggings:      projectLoggings,
		serviceAccounts:      cluster.Core.ServiceAccounts(loggingconfig.LoggingNamespace),
	}
	projectLoggings.AddClusterScopedLifecycle("project-logging-controller", cluster.ClusterName, lifecycle)
}

func (c *ProjectLoggingLifecycle) Create(obj *v3.ProjectLogging) (*v3.ProjectLogging, error) {
	newObj, err := v3.LoggingConditionProvisioned.DoUntilTrue(obj, func() (runtime.Object, error) {
		return obj, provision(c.namespaces, c.configmaps, c.serviceAccounts, c.clusterRoleBindings, c.daemonsets, c.clusterLister, c.clusterName)
	})

	return newObj.(*v3.ProjectLogging), err
}
func (c *ProjectLoggingLifecycle) Updated(obj *v3.ProjectLogging) (*v3.ProjectLogging, error) {
	newObj, err := v3.LoggingConditionProvisioned.DoUntilTrue(obj, func() (runtime.Object, error) {
		return obj, provision(c.namespaces, c.configmaps, c.serviceAccounts, c.clusterRoleBindings, c.daemonsets, c.clusterLister, c.clusterName)
	})
	if err != nil {
		return newObj.(*v3.ProjectLogging), err
	}

	newObj, err = v3.LoggingConditionUpdated.Do(newObj, func() (runtime.Object, error) {
		return obj, c.createOrUpdateProjectConfigMap("")
	})

	if err != nil {
		return newObj.(*v3.ProjectLogging), err
	}

	pl := newObj.(*v3.ProjectLogging)
	pl.Status.AppliedSpec = pl.Spec
	return pl, nil
}

func (c *ProjectLoggingLifecycle) Remove(obj *v3.ProjectLogging) (*v3.ProjectLogging, error) {
	isAllDisable, err := utils.CleanResource(c.namespaces, c.clusterLoggingLister, c.projectLoggings.Controller().Lister())
	if err != nil {
		return obj, err
	}

	if !isAllDisable {
		return obj, c.createOrUpdateProjectConfigMap(obj.Name)
	}

	return obj, nil
}

func (c *ProjectLoggingLifecycle) createOrUpdateProjectConfigMap(excludeName string) error {
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
		if v.Name == excludeName {
			continue
		}
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
