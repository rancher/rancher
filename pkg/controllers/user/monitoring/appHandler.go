package monitoring

import (
	"github.com/rancher/rancher/pkg/monitoring"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	corev1 "github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
)

type appHandler struct {
	cattleAppClient             projectv3.AppInterface
	cattleSecretClient          corev1.SecretInterface
	cattleTemplateVersionClient mgmtv3.CatalogTemplateVersionInterface
	cattleProjectClient         mgmtv3.ProjectInterface
	cattleClusterGraphClient    mgmtv3.ClusterMonitorGraphInterface
	cattleProjectGraphClient    mgmtv3.ProjectMonitorGraphInterface
	cattleMonitorMetricClient   mgmtv3.MonitorMetricInterface
	agentDeploymentClient       appsv1beta2.DeploymentInterface
	agentStatefulSetClient      appsv1beta2.StatefulSetInterface
	agentDaemonSetClient        appsv1beta2.DaemonSetInterface
	agentServiceAccountClient   corev1.ServiceAccountInterface
	agentSecretClient           corev1.SecretInterface
	agentNodeClient             corev1.NodeInterface
	agentNamespaceClient        corev1.NamespaceInterface
}

func (ah *appHandler) withdrawApp(clusterID, appName, appTargetNamespace string) error {
	return monitoring.WithdrawApp(ah.cattleAppClient, monitoring.OwnedAppListOptions(clusterID, appName, appTargetNamespace))
}
