package server

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/auth"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/server/router"
	"github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io"
	apiextensionsv1beta1 "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1beta1"
	"github.com/rancher/wrangler/pkg/generated/controllers/apiregistration.k8s.io"
	apiregistrationv1 "github.com/rancher/wrangler/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	corev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	rbacv1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Server struct {
	*Controllers

	RestConfig *rest.Config

	ClientFactory   *client.Factory
	BaseSchemas     *types.APISchemas
	AccessSetLookup accesscontrol.AccessSetLookup
	SchemaTemplates []schema.Template
	AuthMiddleware  auth.Middleware
	Next            http.Handler
	Router          router.RouterFunc
	PostStartHooks  []func() error
	StartHooks      []StartHook
	DashboardURL    func() string
}

type Controllers struct {
	RestConfig *rest.Config
	K8s        kubernetes.Interface
	Core       corev1.Interface
	RBAC       rbacv1.Interface
	API        apiregistrationv1.Interface
	CRD        apiextensionsv1beta1.Interface
	starters   []start.Starter
}

func (c *Controllers) Start(ctx context.Context) error {
	return start.All(ctx, 5, c.starters...)
}

func RestConfigDefaults(cfg *rest.Config) *rest.Config {
	cfg = rest.CopyConfig(cfg)
	cfg.Timeout = 15 * time.Minute
	cfg.RateLimiter = ratelimit.None

	return cfg
}

func NewController(cfg *rest.Config, opts *generic.FactoryOptions) (*Controllers, error) {
	c := &Controllers{}

	core, err := core.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return nil, err
	}
	c.starters = append(c.starters, core)

	rbac, err := rbac.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return nil, err
	}
	c.starters = append(c.starters, rbac)

	api, err := apiregistration.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return nil, err
	}
	c.starters = append(c.starters, api)

	crd, err := apiextensions.NewFactoryFromConfigWithOptions(cfg, opts)
	if err != nil {
		return nil, err
	}
	c.starters = append(c.starters, crd)

	c.K8s, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	c.Core = core.Core().V1()
	c.RBAC = rbac.Rbac().V1()
	c.API = api.Apiregistration().V1()
	c.CRD = crd.Apiextensions().V1beta1()
	c.RestConfig = cfg

	return c, nil
}

type StartHook func(context.Context, *Server) error
