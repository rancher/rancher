package monitoring

import (
	"context"

	"github.com/rancher/rancher/pkg/monitoring"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, agentContext *config.UserContext) {
	clusterName := agentContext.ClusterName

	logrus.Info("Registering cluster monitoring")

	cattleContext := agentContext.Management

	// app handler
	ah := &appHandler{
		cattleTemplateVersionClient: cattleContext.Management.TemplateVersions(metav1.NamespaceAll),
		cattleAppsGetter:            cattleContext.Project,
		cattleMgmtSecretLister:      cattleContext.Core.Secrets("cattle-system").Controller().Lister(),
		agentUserSecret:             agentContext.Core.Secrets(monitoring.CattleNamespaceName),
		agentNamespacesClient:       agentContext.Core.Namespaces(metav1.NamespaceAll),
		agentNodeLister:             agentContext.Core.Nodes(metav1.NamespaceAll).Controller().Lister(),
		agentServiceAccountGetter:   agentContext.Core.(corev1.ServiceAccountsGetter),
		agentRBACClient:             agentContext.RBAC,
	}

	// cluster handler
	clustersClient := cattleContext.Management.Clusters(metav1.NamespaceAll)
	ch := &clusterHandler{
		ctx:                  ctx,
		clusterName:          clusterName,
		cattleClustersClient: clustersClient,
		agentWorkloadsClient: agentContext.Apps,
		app:                  ah,
	}
	clustersClient.AddHandler(ctx, "user-cluster-monitoring", ch.sync)

	// project handler
	projectsClient := cattleContext.Management.Projects(clusterName)
	ph := &projectHandler{
		ctx:                  ctx,
		clusterName:          clusterName,
		cattleClustersClient: clustersClient,
		cattleProjectsClient: projectsClient,
		agentWorkloadsClient: agentContext.Apps,
		app:                  ah,
	}
	projectsClient.Controller().AddClusterScopedHandler(ctx, "user-project-monitoring", clusterName, ph.sync)
}
