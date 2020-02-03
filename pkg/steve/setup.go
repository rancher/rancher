package steve

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/steve/pkg/dashboard"
	"github.com/rancher/rancher/pkg/steve/pkg/github"
	"github.com/rancher/rancher/pkg/steve/pkg/proxy"
	"github.com/rancher/rancher/pkg/wrangler"
	steve "github.com/rancher/steve/pkg/server"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
)

func Setup(server *steve.Server, config *wrangler.Context) error {
	githubHandler, err := github.NewProxy(config.Core.Secret().Cache(),
		settings.GithubProxyAPIURL.Get(),
		"cattle-system",
		"github")
	if err != nil {
		return err
	}

	cfg := authorizerfactory.DelegatingAuthorizerConfig{
		SubjectAccessReviewClient: config.K8s.AuthorizationV1().SubjectAccessReviews(),
		AllowCacheTTL:             time.Second * time.Duration(settings.AuthorizationCacheTTLSeconds.GetInt()),
		DenyCacheTTL:              time.Second * time.Duration(settings.AuthorizationDenyCacheTTLSeconds.GetInt()),
	}

	authorizer, err := cfg.New()
	if err != nil {
		return err
	}

	server.Next = newRouter(&handler{
		GitHub:   server.AuthMiddleware.Wrap(githubHandler),
		Proxy:    server.AuthMiddleware.Wrap(proxy.NewProxyHandler(authorizer, config.TunnelServer)),
		NotFound: server.Next,
	})

	// wrap with UI
	server.Next = dashboard.Route(server.Next)

	return nil
}

type handler struct {
	GitHub   http.Handler
	Proxy    http.Handler
	NotFound http.Handler
}

func newRouter(h *handler) http.Handler {
	mux := mux.NewRouter()
	mux.Handle("/v1/github{path:.*}", h.GitHub)
	mux.Handle("/{prefix:k8s/clusters/[^/]+}{suffix:.*}", h.Proxy)
	mux.NotFoundHandler = h.NotFound
	return mux
}
