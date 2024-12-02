package managementuserlegacy

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *managementv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	// register controller for API
	cluster.APIAggregation.APIServices("").Controller()
	return nil
}
