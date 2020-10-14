package monitoring

import (
	"context"

	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func RegisterIndexers(ctx context.Context, scaledContext *config.ScaledContext) error {
	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	return prtbInformer.AddIndexers(map[string]cache.IndexFunc{
		prtbBySA: prtbBySAFunc,
	})
}

// Register initializes the controllers and registers
func Register(ctx context.Context, agentContext *config.UserContext) {
	clusterName := agentContext.ClusterName
	logrus.Infof("Registering monitoring for cluster %q", clusterName)

	cattleContext := agentContext.Management
	mgmtContext := cattleContext.Management
	cattleClustersClient := mgmtContext.Clusters(metav1.NamespaceAll)
	cattleProjectsClient := mgmtContext.Projects(clusterName)

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
		agentStatefulSetLister:    agentContext.Apps.StatefulSets(metav1.NamespaceAll).Controller().Lister(),
		agentServiceAccountClient: agentContext.Core.ServiceAccounts(metav1.NamespaceAll),
		agentSecretClient:         agentContext.Core.Secrets(metav1.NamespaceAll),
		agentNodeClient:           agentContext.Core.Nodes(metav1.NamespaceAll),
		agentNamespaceClient:      agentContext.Core.Namespaces(metav1.NamespaceAll),
		systemAccountManager:      systemaccount.NewManager(agentContext.Management),
		projectLister:             mgmtContext.Projects(metav1.NamespaceAll).Controller().Lister(),
		catalogTemplateLister:     mgmtContext.CatalogTemplates(metav1.NamespaceAll).Controller().Lister(),
	}

	// operator handler
	oh := &operatorHandler{
		clusterName:   clusterName,
		clusters:      cattleClustersClient,
		clusterLister: mgmtContext.Clusters(metav1.NamespaceAll).Controller().Lister(),
		app:           ah,
	}
	cattleClustersClient.AddHandler(ctx, "prometheus-operator-handler", oh.syncCluster)
	cattleProjectsClient.Controller().AddClusterScopedHandler(ctx, "prometheus-operator-handler", clusterName, oh.syncProject)

	_, clusterMonitoringNamespace := monitoring.ClusterMonitoringInfo()
	agentClusterMonitoringEndpointClient := agentContext.Core.Endpoints(clusterMonitoringNamespace)
	agentClusterMonitoringEndpointLister := agentContext.Core.Endpoints(clusterMonitoringNamespace).Controller().Lister()
	agentNodeClient := agentContext.Core.Nodes(metav1.NamespaceAll)

	// cluster handler
	ch := &clusterHandler{
		clusterName:          clusterName,
		cattleClustersClient: cattleClustersClient,
		cattleClusterLister:  mgmtContext.Clusters(metav1.NamespaceAll).Controller().Lister(),
		agentEndpointsLister: agentClusterMonitoringEndpointLister,
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

	// project handler
	ph := &projectHandler{
		clusterName:         clusterName,
		clusterLister:       mgmtContext.Clusters(metav1.NamespaceAll).Controller().Lister(),
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

	promIndexes := cache.Indexers{
		promByMemberNamespaceIndex: promsByMemberNamespace,
	}

	promInformer := agentContext.Monitoring.Prometheuses("").Controller().Informer()
	promInformer.AddIndexers(promIndexes)

	cr := ConfigRefreshHandler{
		prometheusClient:  agentContext.Monitoring.Prometheuses(""),
		nsLister:          agentContext.Core.Namespaces("").Controller().Lister(),
		prometheusIndexer: promInformer.GetIndexer(),
	}
	agentContext.Core.Namespaces("").AddHandler(ctx, "project-monitoring-config-refresh", cr.syncNamespace)
	agentContext.Monitoring.Prometheuses("").AddHandler(ctx, "project-monitoring-config-refresh", cr.syncPrometheus)
}
