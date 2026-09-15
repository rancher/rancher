package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/rancher/rancher/pkg/api/norman"
	"github.com/rancher/rancher/pkg/auth/api"
	"github.com/rancher/rancher/pkg/auth/data"
	"github.com/rancher/rancher/pkg/auth/handler"
	"github.com/rancher/rancher/pkg/auth/logout"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/providers/scim"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/utils"
	"github.com/rancher/rancher/pkg/wrangler"
	steveauth "github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type Server struct {
	Authenticator steveauth.Middleware
	Management    func(http.Handler) http.Handler
	scaledContext *config.ScaledContext
}

func NewServer(ctx context.Context, wContext *wrangler.Context, scaledContext *config.ScaledContext, authenticator requests.Authenticator) (*Server, error) {
	authManagement, err := newAPIManagement(ctx, scaledContext, authenticator)
	if err != nil {
		return nil, err
	}

	return &Server{
		Authenticator: requests.ToAuthMiddleware(authenticator),
		Management:    authManagement,
		scaledContext: scaledContext,
	}, nil
}

func newAPIManagement(ctx context.Context, scaledContext *config.ScaledContext, authToken requests.AuthTokenGetter) (steveauth.Middleware, error) {
	// Deprecated. Use /v1-public instead.
	v3PublicAPI, err := publicapi.NewV3Handler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	v1PublicAPI, err := publicapi.NewV1Handler(ctx, scaledContext)
	if err != nil {
		return nil, err
	}

	saml := saml.AuthHandler()

	apiLimit, err := quantityAsInt64(getEnvWithDefault("CATTLE_AUTH_API_BODY_LIMIT", "1Mi"), 1024*1024)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Configuring auth server API body limit to %v bytes", apiLimit)

	logout := logout.NewHandler(ctx, tokens.NewManager(scaledContext.Wrangler))

	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext.Wrangler, logout, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	otherAPIs, err := api.NewNormanServer(ctx, scaledContext, authToken)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		authedTokenAPI := requests.NewAuthenticatedFilter(tokenAPI)
		authedOtherAPIs := requests.NewAuthenticatedFilter(otherAPIs)

		v3Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasPrefix(path, "/v3/identit") || strings.HasPrefix(path, "/v3/token") {
				authedTokenAPI.ServeHTTP(w, r)
			} else if strings.HasPrefix(path, "/v3/authConfig") ||
				strings.HasPrefix(path, "/v3/principal") ||
				strings.HasPrefix(path, "/v3/user") ||
				strings.HasPrefix(path, "/v3/schema") ||
				strings.HasPrefix(path, "/v3/subscribe") {
				authedOtherAPIs.ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})

		routes := authRoutes{
			limit:    utils.APIBodyLimitingHandler(apiLimit),
			saml:     saml,
			v1Public: v1PublicAPI,
			v3:       v3Handler,
			logout:   logout,
			next:     next,
		}
		if features.V3Public.Enabled() {
			routes.v3Public = v3PublicAPI // Deprecated. Use /v1-public instead.
		}
		if features.SCIM.Enabled() {
			routes.scim = scim.NewHandler(scaledContext)
		}

		root := newRootMux(routes)

		p := handler.NewFromAuthConfigInterface(scaledContext.Management.AuthConfigs(""))
		p.RegisterOIDCProviderHandlers(root)

		return root
	}, nil
}

// authRoutes are the handlers served by the auth server's root mux. All
// handlers are required except v3Public and scim, which are feature gated:
// when nil, their paths fall through to next.
type authRoutes struct {
	limit    func(http.Handler) http.Handler
	saml     http.Handler
	v3Public http.Handler
	v1Public http.Handler
	scim     http.Handler
	v3       http.Handler
	logout   http.Handler
	next     http.Handler
}

func newRootMux(routes authRoutes) *http.ServeMux {
	root := http.NewServeMux()

	root.Handle("/v1-saml/", routes.limit(routes.saml))
	if routes.v3Public != nil {
		root.Handle("/v3-public/", routes.limit(routes.v3Public)) // Deprecated. Use /v1-public instead.
	}
	root.Handle("/v1-public/", routes.limit(routes.v1Public))
	if routes.scim != nil {
		root.Handle(fmt.Sprint(scim.URLPrefix, "/"), routes.limit(routes.scim))
	}
	// The multi-cluster management router serves this route too, but it only
	// runs when MCM is enabled. Registering it here keeps logout working when
	// MCM is disabled, as in standalone Harvester.
	root.Handle("POST /v1/logout", routes.limit(requests.NewAuthenticatedFilter(routes.logout)))
	root.Handle("/v3/", routes.v3)
	root.Handle("/", routes.next)

	return root
}

func (s *Server) OnLeader(ctx context.Context) error {
	if s.scaledContext == nil {
		return nil
	}

	management := &config.ManagementContext{
		Management: s.scaledContext.Management,
		Core:       s.scaledContext.Core,
		Wrangler:   s.scaledContext.Wrangler,
	}

	if err := data.AuthConfigs(management); err != nil {
		return fmt.Errorf("failed to add authconfig data: %v", err)
	}

	tokens.StartPurgeDaemon(ctx, management)
	providerrefresh.StartRefreshDaemon(s.scaledContext, management)
	logrus.Infof("Steve auth startup complete")
	return nil
}

func (s *Server) Start(ctx context.Context, leader bool) error {
	if s.scaledContext == nil {
		return nil
	}

	if err := s.scaledContext.Start(ctx); err != nil {
		return err
	}
	if leader {
		return s.OnLeader(ctx)
	}
	return nil
}

func SetXAPICattleAuthHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		user, ok := request.UserFrom(req.Context())
		if ok {
			ok = false
			for _, group := range user.GetGroups() {
				if group == "system:authenticated" {
					ok = true
				}
			}
		}
		rw.Header().Set("X-API-Cattle-Auth", fmt.Sprint(ok))
		next.ServeHTTP(rw, req)
	})
}

func quantityAsInt64(s string, d int64) (int64, error) {
	i, err := resource.ParseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("parsing setting: %w", err)
	}

	q, ok := i.AsInt64()
	if ok {
		return q, nil
	}

	return d, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultValue
}
