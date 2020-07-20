package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/clustercache"
	schemacontroller "github.com/rancher/steve/pkg/controllers/schema"
	"github.com/rancher/steve/pkg/dashboard"
	"github.com/rancher/steve/pkg/resources"
	"github.com/rancher/steve/pkg/resources/common"
	"github.com/rancher/steve/pkg/resources/schemas"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/server/handler"
	"github.com/rancher/steve/pkg/summarycache"
)

var ErrConfigRequired = errors.New("rest config is required")

func setDefaults(server *Server) error {
	if server.RESTConfig == nil {
		return ErrConfigRequired
	}

	if server.Controllers == nil {
		var err error
		server.Controllers, err = NewController(server.RESTConfig, nil)
		if err != nil {
			return err
		}
	}

	if server.Next == nil {
		server.Next = http.NotFoundHandler()
	}

	if server.BaseSchemas == nil {
		server.BaseSchemas = types.EmptyAPISchemas()
	}

	return nil
}

func setup(ctx context.Context, server *Server) (http.Handler, *schema.Collection, error) {
	err := setDefaults(server)
	if err != nil {
		return nil, nil, err
	}

	cf := server.ClientFactory
	if cf == nil {
		cf, err = client.NewFactory(server.RESTConfig, server.AuthMiddleware != nil)
		if err != nil {
			return nil, nil, err
		}
		server.ClientFactory = cf
	}

	asl := server.AccessSetLookup
	if asl == nil {
		asl = accesscontrol.NewAccessStore(ctx, true, server.RBAC)
	}

	ccache := clustercache.NewClusterCache(ctx, cf.DynamicClient())
	server.ClusterCache = ccache

	server.BaseSchemas, err = resources.DefaultSchemas(ctx, server.BaseSchemas, ccache, cf)
	if err != nil {
		return nil, nil, err
	}

	sf := schema.NewCollection(ctx, server.BaseSchemas, asl)
	summaryCache := summarycache.New(sf)
	ccache.OnAdd(ctx, summaryCache.OnAdd)
	ccache.OnRemove(ctx, summaryCache.OnRemove)
	ccache.OnChange(ctx, summaryCache.OnChange)

	server.SchemaTemplates = append(server.SchemaTemplates, resources.DefaultSchemaTemplates(cf, summaryCache, asl, server.K8s.Discovery())...)

	cols, err := common.NewDynamicColumns(server.RESTConfig)
	if err != nil {
		return nil, nil, err
	}

	schemas.SetupWatcher(ctx, server.BaseSchemas, asl, sf)

	sync := schemacontroller.Register(ctx,
		cols,
		server.K8s.Discovery(),
		server.CRD.CustomResourceDefinition(),
		server.API.APIService(),
		server.K8s.AuthorizationV1().SelfSubjectAccessReviews(),
		ccache,
		sf)

	handler, err := handler.New(server.RESTConfig, sf, server.AuthMiddleware, server.Next, server.Router)
	if err != nil {
		return nil, nil, err
	}

	server.PostStartHooks = append(server.PostStartHooks, func() error {
		return sync()
	})

	if server.DashboardURL != nil && server.DashboardURL() != "" {
		handler = dashboard.Route(handler, server.DashboardURL)
	}

	return handler, sf, nil
}

func (c *Server) Handler(ctx context.Context) (http.Handler, error) {
	handler, sf, err := setup(ctx, c)
	if err != nil {
		return nil, err
	}

	c.Next = handler

	for _, hook := range c.StartHooks {
		if err := hook(ctx, c); err != nil {
			return nil, err
		}
	}

	for i := range c.SchemaTemplates {
		sf.AddTemplate(&c.SchemaTemplates[i])
	}

	if err := c.Controllers.Start(ctx); err != nil {
		return nil, err
	}

	for _, hook := range c.PostStartHooks {
		if err := hook(); err != nil {
			return nil, err
		}
	}

	return c.Next, nil
}

func (c *Server) ListenAndServe(ctx context.Context, httpsPort, httpPort int, opts *server.ListenOpts) error {
	handler, err := c.Handler(ctx)
	if err != nil {
		return err
	}

	if opts == nil {
		opts = &server.ListenOpts{}
	}
	if opts.Storage == nil && opts.Secrets == nil {
		opts.Secrets = c.Core.Secret()
	}
	if err := server.ListenAndServe(ctx, httpsPort, httpPort, handler, opts); err != nil {
		return err
	}

	if err := c.Controllers.Start(ctx); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}
