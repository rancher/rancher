package logging

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
)

// ClusterLoggingSyncer listens for clusterLogging CRD in management API
// and update the changes to configmap, deploy fluentd, embedded elasticsearch, embedded kibana

type ClusterLoggingSyncer struct {
	clusterName          string
	clusterLoggings      v3.ClusterLoggingInterface
	clusterLoggingLister v3.ClusterLoggingLister
	clusterRoleBindings  rbacv1.ClusterRoleBindingInterface
	clusterLister        v3.ClusterLister
	configmaps           v1.ConfigMapInterface
	daemonsets           v1beta2.DaemonSetInterface
	deployments          v1beta2.DeploymentInterface
	deploymentLister     v1beta2.DeploymentLister
	endpointLister       v1.EndpointsLister
	k8sNodeLister        v1.NodeLister
	namespaces           v1.NamespaceInterface
	nodeLister           v3.NodeLister
	projectLoggingLister v3.ProjectLoggingLister
	roles                rbacv1.RoleInterface
	rolebindings         rbacv1.RoleBindingInterface
	services             v1.ServiceInterface
	serviceLister        v1.ServiceLister
	serviceAccounts      v1.ServiceAccountInterface
}

func registerClusterLogging(cluster *config.UserContext) {
	clusterloggingClient := cluster.Management.Management.ClusterLoggings(cluster.ClusterName)
	syncer := &ClusterLoggingSyncer{
		clusterName:          cluster.ClusterName,
		clusterLoggings:      clusterloggingClient,
		clusterLoggingLister: clusterloggingClient.Controller().Lister(),
		clusterRoleBindings:  cluster.RBAC.ClusterRoleBindings(loggingconfig.LoggingNamespace),
		clusterLister:        cluster.Management.Management.Clusters("").Controller().Lister(),
		configmaps:           cluster.Core.ConfigMaps(loggingconfig.LoggingNamespace),
		daemonsets:           cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace),
		deployments:          cluster.Apps.Deployments(loggingconfig.LoggingNamespace),
		deploymentLister:     cluster.Apps.Deployments("").Controller().Lister(),
		k8sNodeLister:        cluster.Core.Nodes("").Controller().Lister(),
		namespaces:           cluster.Core.Namespaces(""),
		nodeLister:           cluster.Management.Management.Nodes("").Controller().Lister(),
		endpointLister:       cluster.Core.Endpoints("").Controller().Lister(),
		projectLoggingLister: cluster.Management.Management.ProjectLoggings("").Controller().Lister(),
		roles:                cluster.RBAC.Roles(loggingconfig.LoggingNamespace),
		rolebindings:         cluster.RBAC.RoleBindings(loggingconfig.LoggingNamespace),
		services:             cluster.Core.Services(loggingconfig.LoggingNamespace),
		serviceLister:        cluster.Core.Services("").Controller().Lister(),
		serviceAccounts:      cluster.Core.ServiceAccounts(loggingconfig.LoggingNamespace),
	}
	clusterloggingClient.AddClusterScopedHandler("cluster-logging-controller", cluster.ClusterName, syncer.Sync)
}

func (c *ClusterLoggingSyncer) Sync(key string, obj *v3.ClusterLogging) error {
	if obj == nil || obj.DeletionTimestamp != nil || utils.GetClusterTarget(obj.Spec) == "none" {
		isAllDisable, err := utils.CleanResource(c.namespaces, c.clusterLoggingLister, c.projectLoggingLister, obj, nil)
		if err != nil {
			return err
		}
		if !isAllDisable {
			utils.UnsetConfigMap(c.configmaps, loggingconfig.ClusterLoggingName, "cluster")
		}

		var updateErr error
		if obj != nil && !reflect.DeepEqual(obj.Spec, obj.Status.AppliedSpec) {
			updatedObj := obj.DeepCopy()
			updatedObj.Status.AppliedSpec = obj.Spec
			_, updateErr = c.clusterLoggings.Update(updatedObj)
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
		if _, err = c.clusterLoggings.Update(newObj); err != nil {
			return err
		}
	}

	return nil
}

func (c *ClusterLoggingSyncer) doSync(obj *v3.ClusterLogging) (*v3.ClusterLogging, error) {
	_, err := v3.LoggingConditionProvisioned.Do(obj, func() (runtime.Object, error) {
		return obj, provision(c.namespaces, c.configmaps, c.serviceAccounts, c.clusterRoleBindings, c.daemonsets, c.clusterLister, c.clusterName)
	})
	if err != nil {
		return obj, err
	}

	if reflect.DeepEqual(obj.Spec, obj.Status.AppliedSpec) {
		return obj, nil
	}

	newObj, err := v3.LoggingConditionUpdated.Do(obj, func() (runtime.Object, error) {
		return c.update(obj)
	})

	if err != nil {
		return newObj.(*v3.ClusterLogging), err
	}

	obj = newObj.(*v3.ClusterLogging)
	obj.Status.AppliedSpec = obj.Spec
	return obj, nil
}

func (c *ClusterLoggingSyncer) createOrUpdateClusterConfigMap() error {
	clusterLoggingList, err := c.clusterLoggings.Controller().Lister().List("", labels.NewSelector())
	if err != nil {
		return errors.Wrap(err, "list cluster logging failed")
	}

	if len(clusterLoggingList) == 0 {
		return errors.New("no cluster logging configured")
	}

	conf := make(map[string]interface{})
	wpClusterlogging, err := utils.ToWrapClusterLogging(clusterLoggingList[0].Spec)
	if err != nil {
		return errors.Wrap(err, "to wraper cluster logging failed")
	}
	conf["clusterTarget"] = wpClusterlogging
	err = generator.GenerateConfigFile(loggingconfig.ClusterConfigPath, generator.ClusterTemplate, "cluster", conf)
	if err != nil {
		return errors.Wrap(err, "generate cluster config file failed")
	}
	return utils.UpdateConfigMap(loggingconfig.ClusterConfigPath, loggingconfig.ClusterLoggingName, "cluster", c.configmaps)
}

func (c *ClusterLoggingSyncer) update(obj *v3.ClusterLogging) (newobj *v3.ClusterLogging, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	logger, io := utils.NewClusterLoggingLogger(obj, c.clusterLoggings, v3.LoggingConditionUpdated)
	defer func() {
		defer io.Close()
		defer cancel()
	}()

	//embedded
	if utils.GetClusterTarget(obj.Spec) != "embedded" {
		if err = utils.RemoveEmbeddedTarget(c.deployments, c.serviceAccounts, c.services, c.roles, c.rolebindings); err != nil {
			return obj, err
		}
		return obj, c.createOrUpdateClusterConfigMap()
	}

	logger.Infof("Start creating embedded %s, %s", loggingconfig.EmbeddedESName, loggingconfig.EmbeddedKibanaName)
	if err = utils.CreateOrUpdateEmbeddedTarget(c.deployments, c.serviceAccounts, c.services, c.roles, c.rolebindings, loggingconfig.LoggingNamespace, obj); err != nil {
		return obj, err
	}

	logger.Infof("Checking embedded components deployment progress")
	var esEndpoint, kibanaEndpoint string
	if esEndpoint, kibanaEndpoint, err = utils.GetEmbeddedEndpointWithRetry(ctx, c.deploymentLister, c.endpointLister, c.serviceLister, c.clusterLoggings, c.nodeLister, c.k8sNodeLister, c.clusterName, logger); err != nil {
		return obj, err
	}

	//return new version cluster logging
	updatedObj, err := c.clusterLoggings.Get(obj.Name, metav1.GetOptions{})
	if err != nil {
		return updatedObj, err
	}
	updatedObj.Spec.EmbeddedConfig.ElasticsearchEndpoint = esEndpoint
	updatedObj.Spec.EmbeddedConfig.KibanaEndpoint = kibanaEndpoint
	return updatedObj, c.createOrUpdateClusterConfigMap()
}

func provision(namespaces v1.NamespaceInterface, configmaps v1.ConfigMapInterface, serviceAccounts v1.ServiceAccountInterface, clusterRoleBindings rbacv1.ClusterRoleBindingInterface, daemonsets v1beta2.DaemonSetInterface, clusterLister v3.ClusterLister, clusterName string) error {
	if err := utils.IniteNamespace(namespaces); err != nil {
		return err
	}

	if err := utils.InitConfigMap(configmaps); err != nil {
		return err
	}

	if err := utils.CreateLogAggregator(daemonsets, serviceAccounts, clusterRoleBindings, clusterLister, clusterName, loggingconfig.LoggingNamespace); err != nil {
		return err
	}
	return utils.CreateFluentd(daemonsets, serviceAccounts, clusterRoleBindings, loggingconfig.LoggingNamespace)
}
