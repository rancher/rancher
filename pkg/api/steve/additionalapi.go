package steve

import (
	"net/http"

	gmux "github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/api/steve/github"
	"github.com/rancher/rancher/pkg/api/steve/health"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

func AdditionalAPIs(config *wrangler.Context) (func(http.Handler) http.Handler, error) {
	githubHandler, err := github.NewProxy(config.Core.Secret().Cache(),
		settings.GithubProxyAPIURL.Get(),
		"cattle-system",
		"github")
	if err != nil {
		return nil, err
	}

	mux := gmux.NewRouter()
	mux.UseEncodedPath()
	mux.Handle("/v1/github{path:.*}", githubHandler)
	health.Register(mux)
	return func(next http.Handler) http.Handler {
		mux.NotFoundHandler = next
		return mux
	}, nil
}
