package monitoring

import (
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/systemaccount"
)

type appHandler struct {
	cattleAppClient           projectv3.AppInterface
	cattleSecretClient        corev1.SecretInterface
	cattleProjectClient       mgmtv3.ProjectInterface
	cattleClusterGraphClient  mgmtv3.ClusterMonitorGraphInterface
	cattleProjectGraphClient  mgmtv3.ProjectMonitorGraphInterface
	cattleMonitorMetricClient mgmtv3.MonitorMetricInterface
	agentDeploymentClient     appsv1.DeploymentInterface
	agentStatefulSetClient    appsv1.StatefulSetInterface
	agentStatefulSetLister    appsv1.StatefulSetLister
	agentServiceAccountClient corev1.ServiceAccountInterface
	agentSecretClient         corev1.SecretInterface
	agentNodeClient           corev1.NodeInterface
	agentNamespaceClient      corev1.NamespaceInterface
	systemAccountManager      *systemaccount.Manager
	projectLister             mgmtv3.ProjectLister
	catalogTemplateLister     mgmtv3.CatalogTemplateLister
}

func (ah *appHandler) withdrawApp(clusterID, appName, appTargetNamespace string) error {
	return monitoring.WithdrawApp(ah.cattleAppClient, monitoring.OwnedAppListOptions(clusterID, appName, appTargetNamespace))
}
