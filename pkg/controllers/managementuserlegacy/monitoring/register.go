package monitoring

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	return prtbInformer.AddIndexers(map[string]cache.IndexFunc{
		prtbBySA: prtbBySAFunc,
	})
}

// Register initializes the controllers and registers
func Register(ctx context.Context, agentContext *config.UserContext) {
	starter := agentContext.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, agentContext)
		return nil
	})
	clusters := agentContext.Management.Management.Clusters("")
	clusters.AddHandler(ctx, "monitoring-deferred", func(key string, obj *v3.Cluster) (runtime.Object, error) {
		if obj == nil {
			return nil, nil
		}
		if obj.Name == agentContext.ClusterName && obj.Spec.EnableClusterMonitoring {
			return obj, starter()
		}
		return obj, nil
	})
	projects := agentContext.Management.Management.Projects(agentContext.ClusterName)
	projects.AddHandler(ctx, "monitoring-deferred", func(key string, obj *v3.Project) (runtime.Object, error) {
		if obj != nil &&
			obj.Spec.EnableProjectMonitoring {
			return obj, starter()
		}
		return obj, nil
	})

	// register prometheus operator handler which needs to run if either monitoring or alerting is enabled
	starterPrometheusOperatorDeferred := agentContext.DeferredStart(ctx, func(ctx context.Context) error {
		registerPrometheusOperatorDeferred(ctx, agentContext)
		return nil
	})
	clusters.AddHandler(ctx, "prometheus-operator-deferred", func(key string, obj *v3.Cluster) (runtime.Object, error) {
		if obj == nil {
			return nil, nil
		}
		if obj.Name == agentContext.ClusterName && (obj.Spec.EnableClusterMonitoring || obj.Spec.EnableClusterAlerting) {
			return obj, starterPrometheusOperatorDeferred()
		}
		return obj, nil
	})
}

func registerDeferred(ctx context.Context, agentContext *config.UserContext) {
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

	_, clusterMonitoringNamespace := monitoring.ClusterMonitoringInfo()
	agentClusterMonitoringEndpointClient := agentContext.Core.Endpoints(clusterMonitoringNamespace)
	agentClusterMonitoringEndpointLister := agentContext.Core.Endpoints(clusterMonitoringNamespace).Controller().Lister()
	agentNodeClient := agentContext.Core.Nodes(metav1.NamespaceAll)

	// cluster handler
	ch := &clusterHandler{
		clusterName:          clusterName,
		cattleClustersClient: cattleClustersClient,
		cattleCatalogManager: cattleContext.CatalogManager,
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
		catalogManager:      cattleContext.CatalogManager,
		cattleProjectClient: cattleProjectsClient,
		prtbIndexer:         prtbInformer.GetIndexer(),
		prtbClient:          mgmtContext.ProjectRoleTemplateBindings(""),
		app:                 ah,
	}
	cattleProjectsClient.Controller().AddClusterScopedHandler(ctx, "project-monitoring-handler", clusterName, ph.sync)
}

func registerPrometheusOperatorDeferred(ctx context.Context, agentContext *config.UserContext) {
	clusterName := agentContext.ClusterName
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
		clusterName:    clusterName,
		clusters:       cattleClustersClient,
		clusterLister:  mgmtContext.Clusters(metav1.NamespaceAll).Controller().Lister(),
		catalogManager: cattleContext.CatalogManager,
		app:            ah,
	}

	cattleClustersClient.AddHandler(ctx, "prometheus-operator-handler", oh.syncCluster)
	cattleProjectsClient.Controller().AddClusterScopedHandler(ctx, "prometheus-operator-handler", clusterName, oh.syncProject)
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
