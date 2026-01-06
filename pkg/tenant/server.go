package tenant

import (
	"context"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/managementuser"
	"github.com/rancher/rancher/pkg/features"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	managementapi "github.com/rancher/rancher/pkg/api/norman/server"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	managementdata "github.com/rancher/rancher/pkg/data/management"
	k8sProxyPkg "github.com/rancher/rancher/pkg/k8sproxy"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"

	steveserver "github.com/rancher/steve/pkg/server"
	"k8s.io/client-go/rest"
)

type Tenant struct {
	sc         *config.ScaledContext
	manager    *clustermanager.Manager
	restConfig *rest.Config
}

const LOCAL = "local"

func (t *Tenant) Register(ctx context.Context, wranglerContext wrangler.Context, opt *steveserver.Options, restConfig *rest.Config) error {
	scaledContext, clusterManager, _, err := multiclustermanager.BuildScaledContext(ctx, &wranglerContext, &multiclustermanager.Options{
		HTTPSListenPort: 0,
	})

	if err != nil {
		return err
	}
	t.sc = scaledContext
	t.restConfig = restConfig

	k8sProxy := k8sProxyPkg.New(scaledContext, scaledContext.Dialer, clusterManager)
	// Although this flow also triggers the registration of userControllersController,
	// in the current tenant implementation the cluster state and related components are not registered, so the registration is skipped.
	// Therefore, there is no issue of duplicate registration in the RegisterController function.
	managementAPI, err := managementapi.New(ctx, scaledContext, clusterManager, k8sProxy, true)

	if err != nil {
		return err
	}

	v3Route := mux.NewRouter()
	v3Route.UseEncodedPath()

	// Authenticated routes
	impersonatingAuth := requests.NewImpersonatingAuth(scaledContext.Wrangler, sar.NewSubjectAccessReview(clusterManager))
	accessControlHandler := rbac.NewAccessControlHandler()

	v3Route.Use(impersonatingAuth.ImpersonationMiddleware)
	v3Route.Use(mux.MiddlewareFunc(accessControlHandler))
	v3Route.Use(requests.NewAuthenticatedFilter)

	v3Route.PathPrefix("/v3/projects").Handler(managementAPI)
	v3Route.PathPrefix("/v3/clusters").Handler(managementAPI)
	v3Route.PathPrefix("/v3/roletemplates").Handler(managementAPI)
	v3Route.PathPrefix("/v3/clusterroletemplatebindings").Handler(managementAPI)
	v3Route.PathPrefix("/v3/projectroletemplatebindings").Handler(managementAPI)
	v3Route.PathPrefix("/v3/globalroles").Handler(managementAPI)
	v3Route.PathPrefix("/v3/globalrolebindings").Handler(managementAPI)

	v3Route.NotFoundHandler = opt.Next
	opt.Next = v3Route

	return nil
}

func (t *Tenant) RegisterController(ctx context.Context, wranglerContext *wrangler.Context) error {

	management, err := t.sc.NewManagementContext()
	if err != nil {
		return errors.Wrap(err, "failed to create management context")
	}

	_, err = managementdata.AddRoles(wranglerContext, management)
	if err != nil {
		return err
	}

	manager := t.sc.ClientGetter.(*clustermanager.Manager)

	auth.RegisterEarly(ctx, management, manager)
	auth.RegisterLate(ctx, management)

	clusterCtx, err := config.NewUserContext(t.sc, *t.restConfig, LOCAL)
	localCluster, err := management.Management.Clusters(LOCAL).Get(LOCAL, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = managementuser.Register(ctx, t.sc, clusterCtx, localCluster, manager)
	if err != nil {
		return err
	}

	return nil
}

func Enabled() bool {
	return !features.MCM.Enabled() && features.Tenant.Enabled()
}
