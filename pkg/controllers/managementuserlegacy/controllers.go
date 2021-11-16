package managementuserlegacy

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/alert"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/approuter"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/cis"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/globaldns"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/helm"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/istio"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/logging"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/monitoring"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/pipeline"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/systemimage"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext, clusterRec *managementv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	helm.Register(ctx, cluster, kubeConfigGetter)
	logging.Register(ctx, cluster)
	cis.Register(ctx, cluster)
	pipeline.Register(ctx, cluster)
	systemimage.Register(ctx, cluster)
	approuter.Register(ctx, cluster)
	alert.Register(ctx, cluster)
	globaldns.Register(ctx, cluster)
	monitoring.Register(ctx, cluster)
	istio.Register(ctx, cluster)

	// register controller for API
	cluster.APIAggregation.APIServices("").Controller()
	return nil
}
