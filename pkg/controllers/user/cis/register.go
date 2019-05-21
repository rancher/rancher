package cis

import (
	"context"
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

	mgmtContext := userContext.Management

	userNSClient := userContext.Core.Namespaces(metav1.NamespaceAll)
	mgmtProjClient := mgmtContext.Management.Projects(clusterName)
	mgmtAppClient := mgmtContext.Project.Apps(metav1.NamespaceAll)
	mgmtTemplateVersionClient := mgmtContext.Management.CatalogTemplateVersions(metav1.NamespaceAll)
	systemAccountManager := systemaccount.NewManager(mgmtContext)

	mgmtClusterClient := userContext.Management.Management.Clusters(metav1.NamespaceAll)
	pods := userContext.Core.Pods(DefaultNamespaceForCis)

	podHandler := &podHandler{
		mgmtClusterClient,
		clusterLister,
		userContext.ClusterName,
	}

	clusterHandler := &clusterHandler{
		mgmtCtxClusterClient:         mgmtClusterClient,
		mgmtCtxProjClient:            mgmtProjClient,
		mgmtCtxAppClient:             mgmtAppClient,
		mgmtCtxTemplateVersionClient: mgmtTemplateVersionClient,
		systemAccountManager:         systemAccountManager,
		userCtx:                      userContext,
		clusterNamespace:             userContext.ClusterName,
		userCtxNSClient:              userNSClient,
	}

	pods.AddHandler(ctx, "podHandler", podHandler.Sync)
	mgmtClusterClient.AddHandler(ctx, "clusterHandler", clusterHandler.Sync)
}
