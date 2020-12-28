package cis

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, management *config.ManagementContext) {
	logrus.Debugf("Registering CIS scheduled scan controller")

	clusterClient := management.Management.Clusters(metav1.NamespaceAll)
	clusterLister := clusterClient.Controller().Lister()

	clusterScanClient := management.Management.ClusterScans(metav1.NamespaceAll)
	clusterScanLister := clusterScanClient.Controller().Lister()

	scheduledScanHandler := newScheduleScanHandler(
		management.Management,
		clusterClient,
		clusterLister,
		clusterScanClient,
		clusterScanLister,
	)

	clusterClient.AddHandler(ctx, "scheduledScanHandler:clusterSync", scheduledScanHandler.clusterSync)
	clusterScanClient.AddHandler(ctx, "scheduledScanHandler:sync", scheduledScanHandler.sync)
}
