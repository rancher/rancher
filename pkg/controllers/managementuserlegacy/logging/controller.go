package logging

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/logging/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/logging/deployer"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/logging/watcher"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})
	AddStarter(ctx, cluster, starter)
}

func AddStarter(ctx context.Context, cluster *config.UserContext, starter func() error) {
	clusterLogging := cluster.Management.Management.ClusterLoggings("")
	projectLogging := cluster.Management.Management.ProjectLoggings("")
	clusterLogging.AddClusterScopedHandler(ctx, "logging-deferred", cluster.ClusterName, func(key string, obj *v3.ClusterLogging) (runtime.Object, error) {
		return obj, starter()
	})
	projectLogging.AddClusterScopedHandler(ctx, "logging-deferred", cluster.ClusterName, func(key string, obj *v3.ProjectLogging) (runtime.Object, error) {
		return obj, starter()
	})
}

func registerDeferred(ctx context.Context, cluster *config.UserContext) {

	clusterName := cluster.ClusterName
	secretManager := configsyncer.NewSecretManager(cluster)

	clusterLogging := cluster.Management.Management.ClusterLoggings(clusterName)
	projectLogging := cluster.Management.Management.ProjectLoggings(metav1.NamespaceAll)
	clusterClient := cluster.Management.Management.Clusters(metav1.NamespaceAll)
	node := cluster.Core.Nodes(metav1.NamespaceAll)

	deployer := deployer.NewDeployer(cluster, secretManager)
	clusterLogging.AddClusterScopedHandler(ctx, "cluster-logging-deployer", cluster.ClusterName, deployer.ClusterLoggingSync)
	projectLogging.AddClusterScopedHandler(ctx, "project-logging-deployer", cluster.ClusterName, deployer.ProjectLoggingSync)
	clusterClient.AddHandler(ctx, "cluster-trigger-logging-deployer-updator", deployer.ClusterSync)
	node.AddHandler(ctx, "node-syncer", deployer.NodeSync)

	configSyncer := configsyncer.NewConfigSyncer(cluster, secretManager)
	clusterLogging.AddClusterScopedHandler(ctx, "cluster-logging-configsyncer", cluster.ClusterName, configSyncer.ClusterLoggingSync)
	projectLogging.AddClusterScopedHandler(ctx, "project-logging-configsyncer", cluster.ClusterName, configSyncer.ProjectLoggingSync)

	namespaces := cluster.Core.Namespaces(metav1.NamespaceAll)
	namespaces.AddClusterScopedHandler(ctx, "namespace-logging-configsysncer", cluster.ClusterName, configSyncer.NamespaceSync)

	watcher.StartEndpointWatcher(ctx, cluster)
}
