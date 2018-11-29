package monitoring

import (
	"context"

	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, agentContext *config.UserContext) {
	clusterName := agentContext.ClusterName
	logrus.Infof("Registering monitoring for cluster %q", clusterName)

	cattleContext := agentContext.Management
	clustersClient := cattleContext.Management.Clusters(metav1.NamespaceAll)
	projectsClient := cattleContext.Management.Projects(clusterName)

	// app handler
	ah := &appHandler{
		cattleTemplateVersionsGetter: cattleContext.Management,
		cattleProjectsGetter:         cattleContext.Management,
		cattleAppsGetter:             cattleContext.Project,
		cattleCoreClient:             cattleContext.Core,
		agentCoreClient:              agentContext.Core,
		agentRBACClient:              agentContext.RBAC,
		agentWorkloadsClient:         agentContext.Apps,
	}

	// cluster handler

	ch := &clusterHandler{
		ctx:                  ctx,
		clusterName:          clusterName,
		cattleClustersClient: clustersClient,
		app:                  ah,
		clusterGraph:         cattleContext.Management.ClusterMonitorGraphs(""),
		monitorMetrics:       cattleContext.Management.MonitorMetrics(""),
	}
	clustersClient.AddHandler(ctx, "user-cluster-monitoring", ch.sync)

	// project handler
	ph := &projectHandler{
		ctx:                  ctx,
		clusterName:          clusterName,
		cattleClustersClient: clustersClient,
		cattleProjectsClient: projectsClient,
		app:                  ah,
		projectGraph:         cattleContext.Management.ProjectMonitorGraphs(""),
	}
	projectsClient.Controller().AddClusterScopedHandler(ctx, "user-project-monitoring", clusterName, ph.sync)
}
