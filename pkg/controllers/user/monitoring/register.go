package monitoring

import (
	"context"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, agentContext *config.UserContext) {
	clusterName := agentContext.ClusterName
	logrus.Infof("Registering monitoring for cluster %q", clusterName)

	cattleContext := agentContext.Management
	mgmtContext := cattleContext.Management
	cattleClustersClient := mgmtContext.Clusters(metav1.NamespaceAll)
	cattleProjectsClient := mgmtContext.Projects(clusterName)
	agentNodeClient := agentContext.Core.Nodes(metav1.NamespaceAll)

	_, clusterMonitoringNamespace := monitoring.ClusterMonitoringInfo()
	agentClusterMonitoringEndpointClient := agentContext.Core.Endpoints(clusterMonitoringNamespace)
	agentClusterMonitoringEndpointLister := agentClusterMonitoringEndpointClient.Controller().Lister()

	// app handler
	ah := &appHandler{
		cattleAppClient:           cattleContext.Project.Apps(metav1.NamespaceAll),
		cattleProjectClient:       cattleProjectsClient,
		cattleSecretClient:        cattleContext.Core.Secrets(metav1.NamespaceAll),
		cattleClusterGraphClient:  mgmtContext.ClusterMonitorGraphs(metav1.NamespaceAll),
		cattleProjectGraphClient:  mgmtContext.ProjectMonitorGraphs(metav1.NamespaceAll),
		cattleMonitorMetricClient: mgmtContext.MonitorMetrics(metav1.NamespaceAll),
		agentDeploymentClient:     agentContext.Apps.Deployments(metav1.NamespaceAll),
		agentStatefulSetClient:    agentContext.Apps.StatefulSets(metav1.NamespaceAll),
		agentServiceAccountClient: agentContext.Core.ServiceAccounts(metav1.NamespaceAll),
		agentSecretClient:         agentContext.Core.Secrets(metav1.NamespaceAll),
		agentNodeClient:           agentNodeClient,
		agentNamespaceClient:      agentContext.Core.Namespaces(metav1.NamespaceAll),
		agentEndpointClient:       agentContext.Core.Endpoints(metav1.NamespaceAll),
		agentEndpointsForCluster:  agentClusterMonitoringEndpointLister,
		systemAccountManager:      systemaccount.NewManager(agentContext.Management),
		projectLister:             mgmtContext.Projects(metav1.NamespaceAll).Controller().Lister(),
		catalogTemplateLister:     mgmtContext.CatalogTemplates(metav1.NamespaceAll).Controller().Lister(),
	}

	// operator handler
	oh := &operatorHandler{
		clusterName:         clusterName,
		cattleClusterClient: cattleClustersClient,
		app:                 ah,
	}
	cattleClustersClient.AddHandler(ctx, "prometheus-operator-handler", oh.syncCluster)
	cattleProjectsClient.Controller().AddClusterScopedHandler(ctx, "prometheus-operator-handler", clusterName, oh.syncProject)

	// cluster handler
	ch := &clusterHandler{
		clusterName:          clusterName,
		cattleClustersClient: cattleClustersClient,
		app:                  ah,
	}
	cattleClustersClient.AddHandler(ctx, "cluster-monitoring-handler", ch.sync)

	// cluster monitoring enabled handler
	cattleClusterController := cattleClustersClient.Controller()
	cmeh := &clusterMonitoringEnabledHandler{
		clusterName:             clusterName,
		cattleClusterController: cattleClusterController,
		cattleClusterLister:     cattleClusterController.Lister(),
		agentEndpointsLister:    agentClusterMonitoringEndpointLister,
	}
	agentClusterMonitoringEndpointClient.AddHandler(ctx, "cluster-monitoring-enabled-handler", cmeh.sync)
	agentNodeClient.AddHandler(ctx, "cluster-monitoring-sync-windows-node-handler", cmeh.syncWindowsNode)

	prtbInformer := mgmtContext.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbInformer.AddIndexers(map[string]cache.IndexFunc{
		prtbBySA: prtbBySAFunc,
	})

	// project handler
	ph := &projectHandler{
		clusterName:         clusterName,
		cattleClusterClient: cattleClustersClient,
		cattleProjectClient: cattleProjectsClient,
		prtbIndexer:         prtbInformer.GetIndexer(),
		prtbClient:          mgmtContext.ProjectRoleTemplateBindings(""),
		app:                 ah,
	}
	cattleProjectsClient.Controller().AddClusterScopedHandler(ctx, "project-monitoring-handler", clusterName, ph.sync)
}

func RegisterAgent(ctx context.Context, agentContext *config.UserOnlyContext) {
	cp := &ExporterEndpointController{
		Endpoints:           agentContext.Core.Endpoints("cattle-prometheus"),
		EndpointLister:      agentContext.Core.Endpoints("cattle-prometheus").Controller().Lister(),
		EndpointsController: agentContext.Core.Endpoints("cattle-prometheus").Controller(),
		NodeLister:          agentContext.Core.Nodes("").Controller().Lister(),
		ServiceLister:       agentContext.Core.Services("cattle-prometheus").Controller().Lister(),
	}
	agentContext.Core.Nodes("").AddHandler(ctx, "control-plane-endpoint", cp.sync)
	agentContext.Core.Endpoints("cattle-prometheus").AddHandler(ctx, "control-plane-endpoint", cp.syncEndpoints)

	agentContext.Core.Namespaces("").AddHandler(ctx, "project-monitoring-config-refresh", func(_ string, obj *v1.Namespace) (object runtime.Object, err error) {
		// nothing to do, don't block the finalizers after dropping project-level Prometheus
		return obj, nil
	})
	agentContext.Monitoring.Prometheuses("").AddHandler(ctx, "project-monitoring-config-refresh", func(_ string, obj *monitoringv1.Prometheus) (object runtime.Object, err error) {
		// nothing to do, don't block the finalizers after dropping project-level Prometheus
		return obj, nil
	})
}
