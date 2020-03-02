package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/steve/pkg/schemaserver/urlbuilder"
)

type RouterFunc func(h Handlers) http.Handler

type Handlers struct {
	K8sResource     http.Handler
	GenericResource http.Handler
	APIRoot         http.Handler
	K8sProxy        http.Handler
	Next            http.Handler
}

func Routes(h Handlers) http.Handler {
	m := mux.NewRouter()
	m.UseEncodedPath()
	m.StrictSlash(true)
	m.Use(urlbuilder.RedirectRewrite)

	m.Path("/").Handler(h.APIRoot).HeadersRegexp("Accepts", ".*json.*")
	m.Path("/{name:v1}").Handler(h.APIRoot)

	m.Path("/v1/{group}.{version}.{resource}").Handler(h.K8sResource)
	m.Path("/v1/{group}.{version}.{resource}/{nameorns}").Handler(h.K8sResource)
	m.Path("/v1/{group}.{version}.{resource}/{namespace}/{name}").Handler(h.K8sResource)
	m.Path("/v1/{group}.{version}.{resource}/{nameorns}").Queries("action", "{action}").Handler(h.K8sResource)
	m.Path("/v1/{group}.{version}.{resource}/{namespace}/{name}").Queries("action", "{action}").Handler(h.K8sResource)
	m.Path("/v1/{type:schemas}/{name:.*}").Handler(h.GenericResource)
	m.Path("/v1/{type}").Handler(h.GenericResource)
	m.Path("/v1/{type}/{name}").Handler(h.GenericResource)
	m.Path("/api").Handler(h.K8sProxy)
	m.PathPrefix("/api/").Handler(h.K8sProxy)
	m.PathPrefix("/openapi").Handler(h.K8sProxy)
	m.PathPrefix("/version").Handler(h.K8sProxy)
	m.NotFoundHandler = h.Next

	return m
}
