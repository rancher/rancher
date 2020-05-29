package user

import (
	"context"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/rancher/pkg/controllers/user/alert"
	"github.com/rancher/rancher/pkg/controllers/user/approuter"
	"github.com/rancher/rancher/pkg/controllers/user/certsexpiration"
	"github.com/rancher/rancher/pkg/controllers/user/cis"
	"github.com/rancher/rancher/pkg/controllers/user/clusterauthtoken"
	"github.com/rancher/rancher/pkg/controllers/user/dnsrecord"
	"github.com/rancher/rancher/pkg/controllers/user/endpoints"
	"github.com/rancher/rancher/pkg/controllers/user/externalservice"
	"github.com/rancher/rancher/pkg/controllers/user/globaldns"
	"github.com/rancher/rancher/pkg/controllers/user/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/ingress"
	"github.com/rancher/rancher/pkg/controllers/user/ingresshostgen"
	"github.com/rancher/rancher/pkg/controllers/user/istio"
	"github.com/rancher/rancher/pkg/controllers/user/logging"
	"github.com/rancher/rancher/pkg/controllers/user/monitoring"
	"github.com/rancher/rancher/pkg/controllers/user/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/user/noderemove"
	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/controllers/user/nsserviceaccount"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline"
	"github.com/rancher/rancher/pkg/controllers/user/rbac"
	"github.com/rancher/rancher/pkg/controllers/user/rbac/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/user/resourcequota"
	"github.com/rancher/rancher/pkg/controllers/user/secret"
	"github.com/rancher/rancher/pkg/controllers/user/servicemonitor"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/controllers/user/targetworkloadservice"
	"github.com/rancher/rancher/pkg/controllers/user/windows"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	pkgmonitoring "github.com/rancher/rancher/pkg/monitoring"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/factory"
)

func Register(ctx context.Context, cluster *config.UserContext, clusterRec *managementv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	rbac.Register(ctx, cluster)
	healthsyncer.Register(ctx, cluster)
	helm.Register(ctx, cluster, kubeConfigGetter)
	logging.Register(ctx, cluster)
	networkpolicy.Register(ctx, cluster)
	cis.Register(ctx, cluster)
	noderemove.Register(ctx, cluster)
	nodesyncer.Register(ctx, cluster, kubeConfigGetter)
	pipeline.Register(ctx, cluster)
	podsecuritypolicy.RegisterCluster(ctx, cluster)
	podsecuritypolicy.RegisterClusterRole(ctx, cluster)
	podsecuritypolicy.RegisterBindings(ctx, cluster)
	podsecuritypolicy.RegisterNamespace(ctx, cluster)
	podsecuritypolicy.RegisterPodSecurityPolicy(ctx, cluster)
	podsecuritypolicy.RegisterServiceAccount(ctx, cluster)
	podsecuritypolicy.RegisterTemplate(ctx, cluster)
	secret.Register(ctx, cluster)
	systemimage.Register(ctx, cluster)
	endpoints.Register(ctx, cluster)
	approuter.Register(ctx, cluster)
	resourcequota.Register(ctx, cluster)
	globaldns.Register(ctx, cluster)
	alert.Register(ctx, cluster)
	monitoring.Register(ctx, cluster)
	istio.Register(ctx, cluster)
	certsexpiration.Register(ctx, cluster)
	ingresshostgen.Register(ctx, cluster.UserOnlyContext())
	windows.Register(ctx, clusterRec, cluster)
	nsserviceaccount.Register(ctx, cluster)

	// register controller for API
	cluster.APIAggregation.APIServices("").Controller()

	if clusterRec.Spec.LocalClusterAuthEndpoint.Enabled {
		err := clusterauthtoken.CRDSetup(ctx, cluster.UserOnlyContext())
		if err != nil {
			return err
		}
		clusterauthtoken.Register(ctx, cluster)
	}

	if clusterRec.Spec.Internal {
		err := RegisterUserOnly(ctx, cluster.UserOnlyContext())
		if err != nil {
			return err
		}
	}

	return nil
}

func RegisterFollower(ctx context.Context, cluster *config.UserContext, kubeConfigGetter common.KubeConfigGetter, clusterManager healthsyncer.ClusterControllerLifecycle) error {
	cluster.Core.Namespaces("").Controller()
	cluster.Core.Services("").Controller()
	cluster.RBAC.ClusterRoleBindings("").Controller()
	cluster.RBAC.RoleBindings("").Controller()
	cluster.Core.Endpoints("").Controller()
	cluster.APIAggregation.APIServices("").Controller()
	return nil
}

func RegisterUserOnly(ctx context.Context, cluster *config.UserOnlyContext) error {
	if err := createUserClusterCRDs(ctx, cluster); err != nil {
		return err
	}

	dnsrecord.Register(ctx, cluster)
	externalservice.Register(ctx, cluster)
	ingress.Register(ctx, cluster)
	nslabels.Register(ctx, cluster)
	targetworkloadservice.Register(ctx, cluster)
	workload.Register(ctx, cluster)
	servicemonitor.Register(ctx, cluster)
	monitoring.RegisterAgent(ctx, cluster)

	return nil
}

func createUserClusterCRDs(ctx context.Context, c *config.UserOnlyContext) error {
	overrided := struct {
		types.Namespaced
	}{}

	schemas := factory.Schemas(&pkgmonitoring.APIVersion).
		MustImport(&pkgmonitoring.APIVersion, monitoringv1.Prometheus{}, overrided).
		MustImport(&pkgmonitoring.APIVersion, monitoringv1.PrometheusRule{}, overrided).
		MustImport(&pkgmonitoring.APIVersion, monitoringv1.ServiceMonitor{}, overrided).
		MustImport(&pkgmonitoring.APIVersion, monitoringv1.Alertmanager{}, overrided)

	f, err := crd.NewFactoryFromClient(c.RESTConfig)
	if err != nil {
		return err
	}

	_, err = f.CreateCRDs(ctx, config.UserStorageContext,
		schemas.Schema(&pkgmonitoring.APIVersion, projectclient.PrometheusType),
		schemas.Schema(&pkgmonitoring.APIVersion, projectclient.PrometheusRuleType),
		schemas.Schema(&pkgmonitoring.APIVersion, projectclient.AlertmanagerType),
		schemas.Schema(&pkgmonitoring.APIVersion, projectclient.ServiceMonitorType),
	)

	f.BatchWait()

	return err
}
