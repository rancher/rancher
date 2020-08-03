package rancher

import (
	"context"
	"net/http"

	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	steveapi "github.com/rancher/rancher/pkg/api/steve"
	"github.com/rancher/rancher/pkg/api/steve/clusterapi"
	"github.com/rancher/rancher/pkg/api/steve/proxy"
	"github.com/rancher/rancher/pkg/auth"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/controllers/dashboard"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi"
	crds "github.com/rancher/rancher/pkg/crds/dashboard"
	dashboarddata "github.com/rancher/rancher/pkg/data/dashboard"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/ui"
	"github.com/rancher/rancher/pkg/websocket"
	"github.com/rancher/rancher/pkg/wrangler"
	steveauth "github.com/rancher/steve/pkg/auth"
	steveserver "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/k8scheck"
	"github.com/urfave/cli"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Options struct {
	ACMEDomains       cli.StringSlice
	AddLocal          string
	Embedded          bool
	HTTPListenPort    int
	HTTPSListenPort   int
	K8sMode           string
	Debug             bool
	Trace             bool
	NoCACerts         bool
	AuditLogPath      string
	AuditLogMaxage    int
	AuditLogMaxsize   int
	AuditLogMaxbackup int
	AuditLevel        int
	Features          string
}

type Rancher struct {
	Auth     steveauth.Middleware
	Handler  http.Handler
	Wrangler *wrangler.Context
	Steve    *steveserver.Server

	auditLog   *audit.LogWriter
	authServer *auth.Server
	opts       *Options
}

func New(ctx context.Context, clientConfig clientcmd.ClientConfig, opts *Options) (*Rancher, error) {
	if opts == nil {
		opts = &Options{}
	}

	restConfig, err := setupAndValidationRESTConfig(ctx, clientConfig)
	if err != nil {
		return nil, err
	}

	wranglerContext, err := wrangler.NewContext(ctx, restConfig)
	if err != nil {
		return nil, err
	}

	if err := crds.Create(ctx, restConfig); err != nil {
		return nil, err
	}

	// Initialize Features as early as possible
	features.InitializeFeatures(wranglerContext.Mgmt.Feature(), opts.Features)

	authServer, err := auth.NewServer(ctx, restConfig)
	if err != nil {
		return nil, err
	}

	steve, err := steveserver.New(ctx, restConfig, &steveserver.Options{
		Controllers:     wranglerContext.Controllers,
		AccessSetLookup: wranglerContext.ASL,
		AuthMiddleware:  steveauth.ExistingContext,
		Next:            ui.New(),
	})
	if err != nil {
		return nil, err
	}

	clusterProxy, err := proxy.NewProxyMiddleware(wranglerContext.K8s.AuthorizationV1().SubjectAccessReviews(),
		wranglerContext.MultiClusterManager,
		wranglerContext.Mgmt.Cluster().Cache(),
		localClusterEnabled(opts),
		steve,
	)
	if err != nil {
		return nil, err
	}

	additionalAPI, err := steveapi.AdditionalAPIs(wranglerContext)
	if err != nil {
		return nil, err
	}

	clusterAPI, err := clusterapi.ClusterAPI(ctx, steve)
	if err != nil {
		return nil, err
	}

	wranglerContext.MultiClusterManager = newMCM(wranglerContext, opts)
	auditLogWriter := audit.NewLogWriter(opts.AuditLogPath, opts.AuditLevel, opts.AuditLogMaxage, opts.AuditLogMaxbackup, opts.AuditLogMaxsize)
	auditFilter := audit.NewAuditLogMiddleware(auditLogWriter)

	return &Rancher{
		Auth: authServer.Authenticator.Chain(
			auditFilter),
		Handler: responsewriter.Chain{
			responsewriter.ContentTypeOptions,
			websocket.NewWebsocketHandler,
			proxy.RewriteLocalCluster,
			clusterProxy,
			wranglerContext.MultiClusterManager.Middleware,
			authServer.Management,
			additionalAPI,
			clusterAPI,
		}.Handler(steve),
		Wrangler:   wranglerContext,
		Steve:      steve,
		auditLog:   auditLogWriter,
		authServer: authServer,
		opts:       opts,
	}, nil
}

func (r *Rancher) Start(ctx context.Context) error {
	if err := dashboarddata.EarlyData(ctx, r.Wrangler.K8s); err != nil {
		return err
	}

	if err := dashboardapi.Register(ctx, r.Wrangler); err != nil {
		return err
	}

	if err := steveapi.Setup(ctx, r.Steve, r.Wrangler); err != nil {
		return err
	}

	r.Wrangler.OnLeader(func(ctx context.Context) error {
		return r.Wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
			if err := dashboarddata.Add(ctx, r.Wrangler, localClusterEnabled(r.opts), r.opts.AddLocal == "false", r.opts.Embedded); err != nil {
				return err
			}
			return dashboard.Register(ctx, r.Wrangler)
		})
	})

	if err := r.authServer.Start(ctx, false); err != nil {
		return err
	}

	r.Wrangler.OnLeader(r.authServer.OnLeader)
	r.auditLog.Start(ctx)
	return r.Wrangler.Start(ctx)
}

func (r *Rancher) ListenAndServe(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}

	if err := tls.ListenAndServe(ctx, r.Wrangler.RESTConfig,
		r.Auth(r.Handler),
		r.opts.HTTPSListenPort,
		r.opts.HTTPListenPort,
		r.opts.ACMEDomains,
		r.opts.NoCACerts); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

func newMCM(wrangler *wrangler.Context, opts *Options) wrangler.MultiClusterManager {
	return multiclustermanager.NewDeferredServer(wrangler, &multiclustermanager.Options{
		RemoveLocalCluster:  opts.AddLocal == "false",
		LocalClusterEnabled: localClusterEnabled(opts),
		Embedded:            opts.Embedded,
		HTTPSListenPort:     opts.HTTPSListenPort,
		Debug:               opts.Debug,
		Trace:               opts.Trace,
	})
}

func setupAndValidationRESTConfig(ctx context.Context, clientConfig clientcmd.ClientConfig) (*rest.Config, error) {
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	restConfig = steveserver.RestConfigDefaults(restConfig)
	return restConfig, k8scheck.Wait(ctx, *restConfig)
}

func localClusterEnabled(opts *Options) bool {
	if opts.AddLocal == "true" || (opts.AddLocal == "auto" && !opts.Embedded) {
		return true
	}
	return false
}
