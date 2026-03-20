package projects

import (
	"context"
	"net/http"

	"github.com/rancher/apiserver/pkg/server"
	"github.com/rancher/apiserver/pkg/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/schema"
	steveserver "github.com/rancher/steve/pkg/server"
	"github.com/rancher/steve/pkg/stores/proxy"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

type projectServer struct {
	ctx            context.Context
	asl            accesscontrol.AccessSetLookup
	cf             *client.Factory
	clusterLinks   []string
	namespaceCache corecontrollers.NamespaceCache
}

func Projects(ctx context.Context, config *wrangler.Context, server *steveserver.Server) (func(http.Handler) http.Handler, error) {
	s := projectServer{}
	if err := s.Setup(ctx, config, server); err != nil {
		return nil, err
	}
	return s.middleware(), nil
}

func (s *projectServer) Setup(ctx context.Context, config *wrangler.Context, server *steveserver.Server) error {
	s.ctx = ctx
	s.asl = server.AccessSetLookup
	s.cf = server.ClientFactory
	s.namespaceCache = config.Core.Namespace().Cache()

	server.SchemaFactory.AddTemplate(schema.Template{
		ID: "management.cattle.io.cluster",
		Formatter: func(request *types.APIRequest, resource *types.RawResource) {
			for _, link := range s.clusterLinks {
				resource.Links[link] = request.URLBuilder.Link(resource.Schema, resource.ID, link)
			}
		},
	})

	return nil
}

func (s *projectServer) newSchemas() *types.APISchemas {
	store := proxy.NewProxyStore(s.cf, nil, s.asl, s.namespaceCache)
	schemas := types.EmptyAPISchemas()

	schemas.MustImportAndCustomize(v3.Project{}, func(schema *types.APISchema) {
		schema.Store = store
		attributes.SetNamespaced(schema, true)
		attributes.SetGroup(schema, v3.GroupName)
		attributes.SetVersion(schema, "v3")
		attributes.SetKind(schema, "Project")
		attributes.SetResource(schema, "projects")
		attributes.SetVerbs(schema, []string{"create", "list", "get", "delete", "update", "watch", "patch"})
		s.clusterLinks = append(s.clusterLinks, "projects")
	})

	return schemas
}

func (s *projectServer) newAPIHandler() http.Handler {
	server := server.DefaultAPIServer()
	for k, v := range server.ResponseWriters {
		server.ResponseWriters[k] = stripNS{writer: v}
	}

	s.clusterLinks = []string{
		"subscribe",
		"schemas",
	}

	sf := schema.NewCollection(s.ctx, server.Schemas, s.asl)
	sf.Reset(s.newSchemas().Schemas)

	return schema.WrapServer(sf, server)
}

func (s *projectServer) middleware() func(http.Handler) http.Handler {
	server := s.newAPIHandler()
	server = prefix(server)

	router := http.NewServeMux()

	// Register routes with query parameter checking in handlers
	router.HandleFunc("/v1/management.cattle.io.clusters/{namespace}", func(w http.ResponseWriter, r *http.Request) {
		// Check for ?link=projects or ?link=project query param
		if link := r.URL.Query().Get("link"); link == "projects" || link == "project" {
			server.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	router.Handle("/v1/management.cattle.io.clusters/{namespace}/{type}", server)
	router.Handle("/v1/management.cattle.io.clusters/{namespace}/{type}/{name}", server)
	router.Handle("/v1/management.cattle.io.clusters/{clusterID}/{type}/{namespace}/{name}", server)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try our routes first
			if matches(router, r) {
				router.ServeHTTP(w, r)
				return
			}
			// Fall back to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// matches checks if the request matches any of our patterns
func matches(mux *http.ServeMux, r *http.Request) bool {
	// Check if path starts with our prefix
	return len(r.URL.Path) >= 34 && r.URL.Path[:34] == "/v1/management.cattle.io.clusters/"
}

func prefix(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		clusterID := req.PathValue("clusterID")
		namespace := req.PathValue("namespace")

		if clusterID != "" {
			req.SetPathValue("prefix", "/v1/management.cattle.io.clusters/"+clusterID)
		} else if namespace != "" {
			req.SetPathValue("prefix", "/v1/management.cattle.io.clusters/"+namespace)
		}
		next.ServeHTTP(rw, req)
	})
}
