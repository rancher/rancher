package logging

import (
	"context"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/ticker"
)

const (
	watcherSyncInterval  = 1 * time.Minute
	retryBackoffInterval = 10 * time.Second
	retryTimeout         = 5 * time.Minute
)

// ClusterLoggingSyncer listens for clusterLogging CRD in management API
// and update the changes to config

type ClusterLoggingSyncer struct {
	backoff                  *flowcontrol.Backoff
	clusterName              string
	clusterLoggingController v3.ClusterLoggingController
	clusterLoggings          v3.ClusterLoggingInterface
	clusterLoggingLister     v3.ClusterLoggingLister
	clusterRoleBindings      rbacv1.ClusterRoleBindingInterface
	clusterLister            v3.ClusterLister
	daemonsets               v1beta2.DaemonSetInterface
	deployments              v1beta2.DeploymentInterface
	podLister                v1.PodLister
	k8sNodeLister            v1.NodeLister
	namespaces               v1.NamespaceInterface
	nodeLister               v3.NodeLister
	projectLoggingLister     v3.ProjectLoggingLister
	roles                    rbacv1.RoleInterface
	rolebindings             rbacv1.RoleBindingInterface
	secrets                  v1.SecretInterface
	services                 v1.ServiceInterface
	serviceLister            v1.ServiceLister
	serviceAccounts          v1.ServiceAccountInterface
}

type endpointWatcher struct {
	clusterName          string
	clusterLoggings      v3.ClusterLoggingInterface
	clusterLoggingLister v3.ClusterLoggingLister
	clusterLister        v3.ClusterLister
	deployments          v1beta2.DeploymentInterface
	podLister            v1.PodLister
	k8sNodeLister        v1.NodeLister
	nodeLister           v3.NodeLister
	serviceLister        v1.ServiceLister
}

func registerClusterLogging(ctx context.Context, cluster *config.UserContext) {
	clusterloggingClient := cluster.Management.Management.ClusterLoggings(cluster.ClusterName)
	syncer := &ClusterLoggingSyncer{
		backoff:                  flowcontrol.NewBackOff(retryBackoffInterval, retryTimeout),
		clusterLoggingController: cluster.Management.Management.ClusterLoggings(cluster.ClusterName).Controller(),
		clusterName:              cluster.ClusterName,
		clusterLoggings:          clusterloggingClient,
		clusterLoggingLister:     clusterloggingClient.Controller().Lister(),
		clusterRoleBindings:      cluster.RBAC.ClusterRoleBindings(loggingconfig.LoggingNamespace),
		clusterLister:            cluster.Management.Management.Clusters("").Controller().Lister(),
		daemonsets:               cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace),
		deployments:              cluster.Apps.Deployments(loggingconfig.LoggingNamespace),
		k8sNodeLister:            cluster.Core.Nodes("").Controller().Lister(),
		namespaces:               cluster.Core.Namespaces(""),
		nodeLister:               cluster.Management.Management.Nodes("").Controller().Lister(),
		podLister:                cluster.Core.Pods("").Controller().Lister(),
		projectLoggingLister:     cluster.Management.Management.ProjectLoggings("").Controller().Lister(),
		roles:                    cluster.RBAC.Roles(loggingconfig.LoggingNamespace),
		rolebindings:             cluster.RBAC.RoleBindings(loggingconfig.LoggingNamespace),
		secrets:                  cluster.Core.Secrets(loggingconfig.LoggingNamespace),
		services:                 cluster.Core.Services(loggingconfig.LoggingNamespace),
		serviceLister:            cluster.Core.Services("").Controller().Lister(),
		serviceAccounts:          cluster.Core.ServiceAccounts(loggingconfig.LoggingNamespace),
	}

	endpointWatcher := &endpointWatcher{
		clusterName:          cluster.ClusterName,
		clusterLoggings:      clusterloggingClient,
		clusterLoggingLister: clusterloggingClient.Controller().Lister(),
		k8sNodeLister:        cluster.Core.Nodes("").Controller().Lister(),
		nodeLister:           cluster.Management.Management.Nodes("").Controller().Lister(),
		podLister:            cluster.Core.Pods("").Controller().Lister(),
		serviceLister:        cluster.Core.Services("").Controller().Lister(),
	}

	clusterloggingClient.AddClusterScopedHandler("cluster-logging-controller", cluster.ClusterName, syncer.Sync)

	go endpointWatcher.watch(ctx, watcherSyncInterval)
}

func (e *endpointWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		if err := e.checkTarget(); err != nil {
			logrus.Error(err)
		}
	}
}

func (c *ClusterLoggingSyncer) Sync(key string, obj *v3.ClusterLogging) error {
	if obj == nil || obj.DeletionTimestamp != nil || utils.GetClusterTarget(obj.Spec) == "none" {
		isAllDisable, err := utils.CleanResource(c.namespaces, c.clusterLoggingLister, c.projectLoggingLister, obj, nil)
		if err != nil {
			return err
		}

		if !isAllDisable {
			if err := utils.UnsetSecret(c.secrets, loggingconfig.ClusterLoggingName, "cluster"); err != nil {
				return err
			}

			if err := utils.UnsetSecret(c.secrets, loggingconfig.SSLSecretName, getClusterSecretPrefix(c.clusterName)); err != nil {
				return err
			}
		}

		if obj != nil && !reflect.DeepEqual(obj.Spec, obj.Status.AppliedSpec) {
			return unsetClusterLogging(obj, c.clusterLoggings)
		}
		return nil
	}

	return c.doSync(obj)
}

func (c *ClusterLoggingSyncer) doSync(obj *v3.ClusterLogging) error {
	_, err := v3.LoggingConditionProvisioned.Do(obj, func() (runtime.Object, error) {
		return obj, provision(c.namespaces, c.secrets, c.serviceAccounts, c.clusterRoleBindings, c.daemonsets, c.clusterLister, c.clusterName)
	})
	if err != nil {
		return err
	}

	if reflect.DeepEqual(obj.Spec, obj.Status.AppliedSpec) {
		return nil
	}

	return c.update(obj)
}

func (c *ClusterLoggingSyncer) update(obj *v3.ClusterLogging) (err error) {
	v3.LoggingConditionUpdated.Unknown(obj)

	if err := utils.UpdateSSLAuthentication(getClusterSecretPrefix(c.clusterName), obj.Spec.ElasticsearchConfig, obj.Spec.SplunkConfig, obj.Spec.KafkaConfig, obj.Spec.SyslogConfig, obj.Spec.FluentForwarderConfig, c.secrets); err != nil {
		return err
	}

	return c.createOrUpdateClusterConfig()
}

func (c *ClusterLoggingSyncer) createOrUpdateClusterConfig() error {
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
	conf["clusterName"] = c.clusterName
	err = generator.GenerateConfigFile(loggingconfig.ClusterConfigPath, generator.ClusterTemplate, "cluster", conf)
	if err != nil {
		return errors.Wrap(err, "generate cluster config file failed")
	}

	return utils.UpdateConfig(loggingconfig.ClusterConfigPath, loggingconfig.ClusterLoggingName, "cluster", c.secrets)
}

func (e *endpointWatcher) checkTarget() error {
	cls, err := e.clusterLoggingLister.List(e.clusterName, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "list clusterlogging fail in endpoint watcher")
	}
	if len(cls) == 0 {
		return nil
	}
	obj := cls[0]
	_, err = utils.GetWrapConfig(obj.Spec.ElasticsearchConfig, obj.Spec.SplunkConfig, obj.Spec.SyslogConfig, obj.Spec.KafkaConfig, obj.Spec.FluentForwarderConfig)
	if err != nil {
		return err
	}
	updatedObj := setClusterLoggingErrMsg(obj, err)

	_, updateErr := e.clusterLoggings.Update(updatedObj)

	return errors.Wrapf(updateErr, "set clusterlogging fail in watch endpoint")
}

func provision(namespaces v1.NamespaceInterface, secrets v1.SecretInterface, serviceAccounts v1.ServiceAccountInterface, clusterRoleBindings rbacv1.ClusterRoleBindingInterface, daemonsets v1beta2.DaemonSetInterface, clusterLister v3.ClusterLister, clusterName string) error {
	if err := utils.IniteNamespace(namespaces); err != nil {
		return err
	}

	if err := utils.InitSecret(secrets); err != nil {
		return err
	}

	cluster, err := clusterLister.Get("", clusterName)
	if err != nil {
		return errors.Wrapf(err, "get dockerRootDir from cluster %s failed", clusterName)
	}

	if err := utils.CreateLogAggregator(daemonsets, serviceAccounts, clusterRoleBindings, utils.GetDriverDir(cluster.Status.Driver), loggingconfig.LoggingNamespace); err != nil {
		return err
	}

	return utils.CreateFluentd(daemonsets, serviceAccounts, clusterRoleBindings, loggingconfig.LoggingNamespace, cluster.Spec.DockerRootDir)
}

func unsetClusterLogging(obj *v3.ClusterLogging, clusterLoggings v3.ClusterLoggingInterface) error {
	updatedObj := obj.DeepCopy()

	updatedObj.Status.AppliedSpec = obj.Spec
	updatedObj.Status.FailedSpec = nil

	v3.LoggingConditionProvisioned.False(updatedObj)
	v3.LoggingConditionProvisioned.Message(updatedObj, "")
	v3.LoggingConditionUpdated.False(updatedObj)
	v3.LoggingConditionUpdated.Message(updatedObj, "")

	_, err := clusterLoggings.Update(updatedObj)
	return err
}

func setClusterLoggingErrMsg(obj *v3.ClusterLogging, err error) *v3.ClusterLogging {
	updatedObj := obj.DeepCopy()
	if err != nil {
		updatedObj.Status.FailedSpec = &obj.Spec
		v3.LoggingConditionUpdated.False(updatedObj)
		v3.LoggingConditionUpdated.Message(updatedObj, err.Error())
		return updatedObj
	}

	updatedObj.Status.FailedSpec = nil
	updatedObj.Status.AppliedSpec = obj.Spec

	v3.LoggingConditionUpdated.True(updatedObj)
	v3.LoggingConditionUpdated.Message(updatedObj, "")
	return updatedObj
}

func getClusterSecretPrefix(clusterName string) string {
	return "cluster_" + clusterName
}
