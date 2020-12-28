package logging

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementuser/logging/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/logging/deployer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/logging/watcher"
	"github.com/rancher/rancher/pkg/types/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context, cluster *config.UserContext) {

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
