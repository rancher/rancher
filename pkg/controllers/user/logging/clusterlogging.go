package logging

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/ticker"
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

// ClusterLoggingSyncer listens for clusterLogging CRD in management API
// and update the changes to configmap, deploy fluentd, embedded elasticsearch, embedded kibana

type ClusterLoggingSyncer struct {
	clusterLoggings      v3.ClusterLoggingInterface
	clusterLoggingLister v3.ClusterLoggingLister
	projectLoggingLister v3.ProjectLoggingLister
	podLister            v1.PodLister
	services             v1.ServiceInterface
	serviceLister        v1.ServiceLister
	serviceAccounts      v1.ServiceAccountInterface
	configmaps           v1.ConfigMapInterface
	namespaces           v1.NamespaceInterface
	daemonsets           v1beta2.DaemonSetInterface
	deployments          v1beta2.DeploymentInterface
	roles                rbacv1.RoleInterface
	rolebindings         rbacv1.RoleBindingInterface
	clusterRoleBindings  rbacv1.ClusterRoleBindingInterface
}

func registerClusterLogging(cluster *config.UserContext) {
	clusterloggingClient := cluster.Management.Management.ClusterLoggings(cluster.ClusterName)
	syncer := &ClusterLoggingSyncer{
		clusterLoggings:      clusterloggingClient,
		clusterLoggingLister: clusterloggingClient.Controller().Lister(),
		projectLoggingLister: cluster.Management.Management.ProjectLoggings("").Controller().Lister(),
		podLister:            cluster.Core.Pods("").Controller().Lister(),
		services:             cluster.Core.Services(loggingconfig.LoggingNamespace),
		serviceLister:        cluster.Core.Services("").Controller().Lister(),
		serviceAccounts:      cluster.Core.ServiceAccounts(loggingconfig.LoggingNamespace),
		configmaps:           cluster.Core.ConfigMaps(loggingconfig.LoggingNamespace),
		namespaces:           cluster.Core.Namespaces(""),
		daemonsets:           cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace),
		deployments:          cluster.Apps.Deployments(loggingconfig.LoggingNamespace),
		roles:                cluster.RBAC.Roles(loggingconfig.LoggingNamespace),
		rolebindings:         cluster.RBAC.RoleBindings(loggingconfig.LoggingNamespace),
		clusterRoleBindings:  cluster.RBAC.ClusterRoleBindings(loggingconfig.LoggingNamespace),
	}
	clusterloggingClient.AddClusterScopedHandler("cluster-logging-controller", cluster.ClusterName, syncer.Sync)
}

func (c *ClusterLoggingSyncer) Sync(key string, obj *v3.ClusterLogging) error {
	//clean up
	if obj == nil || obj.DeletionTimestamp != nil {
		if err := utils.RemoveEmbeddedTarget(c.deployments, c.serviceAccounts, c.services, c.roles, c.rolebindings); err != nil {
			return err
		}

		allDisabled, err := utils.IsAllLoggingDisable(c.clusterLoggings.Controller().Lister(), c.projectLoggingLister)
		if err != nil {
			return err
		}

		if allDisabled {
			if err := utils.RemoveFluentd(c.daemonsets, c.serviceAccounts, c.clusterRoleBindings); err != nil {
				return err
			}
			if err := utils.RemoveConfigMap(c.configmaps); err != nil {
				return err
			}
		}
		return nil
	}

	if err := utils.IniteNamespace(c.namespaces); err != nil {
		return err
	}
	if err := utils.InitConfigMap(c.configmaps); err != nil {
		return err
	}
	if utils.GetClusterTarget(obj.Spec) == "embedded" {
		if err := utils.CreateEmbeddedTarget(c.deployments, c.serviceAccounts, c.services, c.roles, c.rolebindings, loggingconfig.LoggingNamespace); err != nil {
			return err
		}

		c.updateEmbeddedEndpoint()
	} else {
		if err := utils.RemoveEmbeddedTarget(c.deployments, c.serviceAccounts, c.services, c.roles, c.rolebindings); err != nil {
			return err
		}
	}

	if err := c.createOrUpdateClusterConfigMap(); err != nil {
		return err
	}
	return utils.CreateFluentd(c.daemonsets, c.serviceAccounts, c.clusterRoleBindings, loggingconfig.LoggingNamespace)
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

func (c *ClusterLoggingSyncer) updateEmbeddedEndpoint() {
	timeout := 2 * time.Minute
	syncInterval := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	go func(ctx context.Context, cancel context.CancelFunc) {
		for range ticker.Context(ctx, syncInterval) {
			err := utils.UpdateEmbeddedEndpoint(c.podLister, c.serviceLister, c.clusterLoggings)
			if err != nil {
				logrus.Debugf("updateEmbeddedEndpoint failed, %v", err)
			} else {
				cancel()
			}
		}
	}(ctx, cancel)
}
