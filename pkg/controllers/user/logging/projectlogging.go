package logging

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/ticker"
)

const (
	ProjectIDAnnotation = "field.cattle.io/projectId"
)

// ProjectLoggingSyncer listens for projectLogging CRD in management API
// and update the changes to configmap, deploy fluentd

type ProjectLoggingSyncer struct {
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

type projectLoggingEndpointWatcher struct {
	projectLoggings v3.ProjectLoggingInterface
}

func registerProjectLogging(ctx context.Context, cluster *config.UserContext) {
	projectLoggings := cluster.Management.Management.ProjectLoggings("")
	syncer := &ProjectLoggingSyncer{
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

	watcher := projectLoggingEndpointWatcher{
		projectLoggings: projectLoggings,
	}

	projectLoggings.AddClusterScopedHandler("project-logging-controller", cluster.ClusterName, syncer.Sync)

	go watcher.watch(ctx, watcherSyncInterval)
}

func (p *projectLoggingEndpointWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		if err := p.checkTarget(); err != nil {
			logrus.Error(err)
		}
	}
}

func (c *ProjectLoggingSyncer) Sync(key string, obj *v3.ProjectLogging) error {
	if obj == nil || obj.DeletionTimestamp != nil || utils.GetProjectTarget(obj.Spec) == "none" {
		isAllDisable, err := utils.CleanResource(c.namespaces, c.clusterLoggingLister, c.projectLoggings.Controller().Lister(), nil, obj)
		if err != nil {
			return err
		}

		if obj != nil && !isAllDisable {
			if err = c.createOrUpdateProjectConfigMap(obj.Name); err != nil {
				return err
			}
		}

		var updateErr error
		if obj != nil && !reflect.DeepEqual(obj.Spec, obj.Status.AppliedSpec) {
			updatedObj := obj.DeepCopy()
			updatedObj.Status.AppliedSpec = obj.Spec
			v3.LoggingConditionProvisioned.False(updatedObj)
			v3.LoggingConditionUpdated.False(updatedObj)
			_, updateErr = c.projectLoggings.Update(updatedObj)
		}
		return updateErr
	}

	original := obj
	obj = original.DeepCopy()

	newObj, err := c.doSync(obj)
	if err != nil {
		return err
	}

	if newObj != nil && !reflect.DeepEqual(newObj, original) {
		if _, err = c.projectLoggings.Update(newObj); err != nil {
			return err
		}
	}

	return nil
}

func (c *ProjectLoggingSyncer) doSync(obj *v3.ProjectLogging) (*v3.ProjectLogging, error) {
	newObj := obj.DeepCopy()
	_, err := v3.LoggingConditionProvisioned.Do(obj, func() (runtime.Object, error) {
		return obj, provision(c.namespaces, c.configmaps, c.serviceAccounts, c.clusterRoleBindings, c.daemonsets, c.clusterLister, c.clusterName)
	})
	if err != nil {
		return obj, err
	}

	_, err = v3.LoggingConditionUpdated.Do(obj, func() (runtime.Object, error) {
		return obj, c.createOrUpdateProjectConfigMap("")
	})

	if err != nil {
		return obj, err
	}

	newObj.Status.AppliedSpec = obj.Spec
	return newObj, nil
}

func (c *ProjectLoggingSyncer) createOrUpdateProjectConfigMap(excludeName string) error {
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

func (p *projectLoggingEndpointWatcher) checkTarget() error {
	pls, err := p.projectLoggings.Controller().Lister().List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "list projectlogging fail in endpoint watcher")
	}
	if len(pls) == 0 {
		return nil
	}

	var mergedErrs error
	for _, v := range pls {
		_, _, err = utils.GetWrapConfig(v.Spec.ElasticsearchConfig, v.Spec.SplunkConfig, v.Spec.SyslogConfig, v.Spec.KafkaConfig, nil)
		if err != nil {
			updatedObj := v.DeepCopy()
			v3.LoggingConditionUpdated.False(updatedObj)
			v3.LoggingConditionUpdated.Message(updatedObj, err.Error())
			_, updateErr := p.projectLoggings.Update(updatedObj)
			mergedErrs = mergedErrors(mergedErrs, errors.Wrapf(err, "%s:%s", v.Namespace, v.Name), updateErr)
		}
	}
	return errors.Wrapf(mergedErrs, "check project logging reachable fail in watch endpoint")
}
