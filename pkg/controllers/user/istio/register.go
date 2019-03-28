package istio

import (
	"context"

	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, agentContext *config.UserContext) {
	clusterName := agentContext.ClusterName
	logrus.Infof("Registering istio for cluster %q", clusterName)

	cattleContext := agentContext.Management
	mgmtContext := cattleContext.Management
	cattleAppClient := cattleContext.Project.Apps("")

	// app handler
	ah := &appHandler{
		istioClusterGraphClient:  mgmtContext.ClusterMonitorGraphs(metav1.NamespaceAll),
		istioMonitorMetricClient: mgmtContext.MonitorMetrics(metav1.NamespaceAll),
		clusterName:              clusterName,
	}

	cattleAppClient.Controller().AddClusterScopedHandler(ctx, "istio-app-handler", clusterName, ah.sync)
}
