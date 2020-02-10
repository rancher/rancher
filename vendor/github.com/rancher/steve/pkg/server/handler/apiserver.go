package handler

import (
	"net/http"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/auth"
	k8sproxy "github.com/rancher/steve/pkg/proxy"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schemaserver/server"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/schemaserver/urlbuilder"
	"github.com/rancher/steve/pkg/server/router"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/rest"
)

func New(cfg *rest.Config, sf schema.Factory, authMiddleware auth.Middleware, next http.Handler, routerFunc router.RouterFunc) (http.Handler, error) {
	var (
		proxy http.Handler
		err   error
	)

	a := &apiServer{
		sf:     sf,
		server: server.DefaultAPIServer(),
	}
	a.server.AccessControl = accesscontrol.NewAccessControl()

	if authMiddleware == nil {
		proxy, err = k8sproxy.Handler("/", cfg)
		if err != nil {
			return nil, err
		}
	} else {
		proxy = k8sproxy.ImpersonatingHandler("/", cfg)
	}

	w := authMiddleware.Wrap
	handlers := router.Handlers{
		Next:            next,
		K8sResource:     w(a.apiHandler(k8sAPI)),
		GenericResource: w(a.apiHandler(nil)),
		K8sProxy:        w(proxy),
		APIRoot:         w(a.apiHandler(apiRoot)),
	}
	if routerFunc == nil {
		return router.Routes(handlers), nil
	}
	return routerFunc(handlers), nil
}

type apiServer struct {
	sf     schema.Factory
	server *server.Server
}

func (a *apiServer) common(rw http.ResponseWriter, req *http.Request) (*types.APIRequest, bool) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		return nil, false
	}

	schemas, err := a.sf.Schemas(user)
	if err != nil {
		logrus.Errorf("HTTP request failed: %v", err)
		rw.Write([]byte(err.Error()))
		rw.WriteHeader(http.StatusInternalServerError)
	}

	urlBuilder, err := urlbuilder.NewPrefixed(req, schemas, "v1")
	if err != nil {
		rw.Write([]byte(err.Error()))
		rw.WriteHeader(http.StatusInternalServerError)
		return nil, false
	}

	return &types.APIRequest{
		Schemas:    schemas,
		Request:    req,
		Response:   rw,
		URLBuilder: urlBuilder,
	}, true
}

type APIFunc func(schema.Factory, *types.APIRequest)

func (a *apiServer) apiHandler(apiFunc APIFunc) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		a.api(rw, req, apiFunc)
	})
}

func (a *apiServer) api(rw http.ResponseWriter, req *http.Request, apiFunc APIFunc) {
	apiOp, ok := a.common(rw, req)
	if ok {
		if apiFunc != nil {
			apiFunc(a.sf, apiOp)
		}
		a.server.Handle(apiOp)
	}
}
