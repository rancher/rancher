package steve

import (
	"context"
	"net/http"

	"github.com/rancher/rancher/pkg/api/steve/aggregation"
	"github.com/rancher/rancher/pkg/api/steve/catalog"
	"github.com/rancher/rancher/pkg/api/steve/github"
	"github.com/rancher/rancher/pkg/api/steve/health"
	"github.com/rancher/rancher/pkg/api/steve/projects"
	"github.com/rancher/rancher/pkg/api/steve/proxy"
	"github.com/rancher/rancher/pkg/capr/configserver"
	"github.com/rancher/rancher/pkg/capr/installer"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/oidc/provider"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	steve "github.com/rancher/steve/pkg/server"
)

func AdditionalAPIsPreMCM(config *wrangler.Context) func(http.Handler) http.Handler {
	if features.RKE2.Enabled() {
		var nextHandler http.Handler
		mux := NewPreMCMMux(PreMCMRoutes{
			ConfigServer: configserver.New(config),
			Installer:    installer.Handler,
			Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if nextHandler != nil {
					nextHandler.ServeHTTP(w, r)
				}
			}),
		})
		return func(next http.Handler) http.Handler {
			nextHandler = next
			return mux
		}
	}

	return func(next http.Handler) http.Handler {
		return next
	}
}

// PreMCMRoutes are the handlers served ahead of the multi-cluster management
// mux. All fields are required.
type PreMCMRoutes struct {
	ConfigServer http.Handler
	Installer    http.Handler
	Next         http.Handler
}

func NewPreMCMMux(routes PreMCMRoutes) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(configserver.ConnectAgent, routes.ConfigServer)
	mux.Handle(configserver.ConnectConfigYamlPath, routes.ConfigServer)
	mux.Handle(configserver.ConnectClusterInfo, routes.ConfigServer)
	mux.Handle(installer.SystemAgentInstallPath, routes.Installer)
	mux.Handle(installer.WindowsRke2InstallPath, routes.Installer)
	mux.Handle("/", routes.Next)
	return mux
}

func AdditionalAPIs(ctx context.Context, config *wrangler.Context, steve *steve.Server) (func(http.Handler) http.Handler, error) {
	clusterAPI, err := projects.Projects(ctx, config, steve)
	if err != nil {
		return nil, err
	}

	githubHandler, err := github.NewProxy(config.Core.Secret().Cache(),
		settings.GithubProxyAPIURL.Get(),
		"cattle-system",
		"github")
	if err != nil {
		return nil, err
	}

	var nextHandler http.Handler
	routes := AdditionalAPIRoutes{
		Github: githubHandler,
		Tunnel: Tunnel(config),
		Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if nextHandler != nil {
				nextHandler.ServeHTTP(w, r)
			}
		}),
	}

	if features.UIExtension.Enabled() {
		routes.RegisterUIPlugins = catalog.RegisterUIPluginHandlers
	}

	if features.OIDCProvider.Enabled() {
		p, err := provider.NewProvider(ctx, exttokenstore.NewSystemFromWrangler(config),
			config.Mgmt.Token().Cache(), config.Mgmt.Token(),
			config.Mgmt.User().Cache(), config.Mgmt.UserAttribute().Cache(),
			config.Core.Secret().Cache(), config.Core.Secret(),
			config.Mgmt.OIDCClient().Cache(), config.Mgmt.OIDCClient(),
			config.Core.Namespace())
		if err != nil {
			return nil, err
		}
		routes.RegisterOIDC = p.RegisterOIDCProviderHandles
	}

	mux := NewAdditionalAPIMux(routes)

	return func(next http.Handler) http.Handler {
		nextHandler = clusterAPI(next)
		return mux
	}, nil
}

// AdditionalAPIRoutes are the handlers served by the extras mux. Github, Tunnel
// and Next are required. RegisterUIPlugins and RegisterOIDC are feature gated:
// when nil, their paths fall through to Next.
type AdditionalAPIRoutes struct {
	Github http.Handler
	Tunnel http.Handler
	Next   http.Handler

	RegisterUIPlugins func(*http.ServeMux)
	RegisterOIDC      func(*http.ServeMux)
}

func NewAdditionalAPIMux(routes AdditionalAPIRoutes) *http.ServeMux {
	mux := http.NewServeMux()
	if routes.RegisterUIPlugins != nil {
		routes.RegisterUIPlugins(mux)
	}
	mux.Handle("/v1/github/{path...}", routes.Github)
	mux.Handle("/v3/connect", routes.Tunnel)

	health.Register(mux)

	if routes.RegisterOIDC != nil {
		routes.RegisterOIDC(mux)
	}

	mux.Handle("/", routes.Next)
	return mux
}

func Tunnel(config *wrangler.Context) http.Handler {
	config.TunnelAuthorizer.Add(proxy.NewAuthorizer(config))
	config.TunnelAuthorizer.Add(aggregation.New(config))
	return config.TunnelServer
}
