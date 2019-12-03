package cis

import (
	"context"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, userContext *config.UserContext) {
	logrus.Infof("Registering CIS controller")

	clusterName := userContext.ClusterName
	clusterLister := userContext.Management.Management.Clusters(metav1.NamespaceAll).Controller().Lister()
	projectLister := userContext.Management.Management.Projects(metav1.NamespaceAll).Controller().Lister()

	mgmtContext := userContext.Management

	userNSClient := userContext.Core.Namespaces(metav1.NamespaceAll)
	userCtxCMLister := userContext.Core.ConfigMaps(metav1.NamespaceAll).Controller().Lister()
	mgmtAppClient := mgmtContext.Project.Apps(metav1.NamespaceAll)
	mgmtTemplateVersionLister := mgmtContext.Management.CatalogTemplateVersions(metav1.NamespaceAll).Controller().Lister()
	systemAccountManager := systemaccount.NewManager(mgmtContext)

	mgmtClusterClient := mgmtContext.Management.Clusters(metav1.NamespaceAll)
	mgmtClusterScanClient := mgmtContext.Management.ClusterScans(clusterName)
	pods := userContext.Core.Pods(v3.DefaultNamespaceForCis)
	configMapsClient := userContext.Core.ConfigMaps(v3.DefaultNamespaceForCis)

	podHandler := &podHandler{
		mgmtClusterScanClient,
	}

	clusterScanHandler := &cisScanHandler{
		mgmtCtxClusterClient:         mgmtClusterClient,
		mgmtCtxAppClient:             mgmtAppClient,
		mgmtCtxTemplateVersionLister: mgmtTemplateVersionLister,
		mgmtCtxClusterScanClient:     mgmtClusterScanClient,
		systemAccountManager:         systemAccountManager,
		clusterNamespace:             userContext.ClusterName,
		userCtxNSClient:              userNSClient,
		userCtxCMLister:              userCtxCMLister,
		clusterLister:                clusterLister,
		projectLister:                projectLister,
		configMapsClient:             configMapsClient,
	}

	pods.AddHandler(ctx, "podHandler", podHandler.Sync)
	mgmtClusterScanClient.AddClusterScopedLifecycle(ctx, "cisScanHandler", clusterName, clusterScanHandler)
}
