package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/api/norman"
	"github.com/rancher/rancher/pkg/auth/api"
	"github.com/rancher/rancher/pkg/auth/data"
	"github.com/rancher/rancher/pkg/auth/handler"
	"github.com/rancher/rancher/pkg/auth/logout"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
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

func NewAlwaysAdmin() (*Server, error) {
	return &Server{
		Authenticator: steveauth.ToMiddleware(steveauth.AuthenticatorFunc(steveauth.AlwaysAdmin)),
		Management: func(next http.Handler) http.Handler {
			return next
		},
	}, nil
}

func NewHeaderAuth() (*Server, error) {
	return &Server{
		Authenticator: steveauth.ToMiddleware(steveauth.AuthenticatorFunc(steveauth.Impersonation)),
		Management: func(next http.Handler) http.Handler {
			return next
		},
	}, nil
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
	privateAPI, err := newPrivateAPI(ctx, scaledContext, authToken)
	if err != nil {
		return nil, err
	}

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

	root := mux.NewRouter()
	root.UseEncodedPath()

	apiLimit, err := quantityAsInt64(getEnvWithDefault("CATTLE_AUTH_API_BODY_LIMIT", "1Mi"), 1024*1024)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Configuring auth server API body limit to %v bytes", apiLimit)

	p := handler.NewFromAuthConfigInterface(scaledContext.Management.AuthConfigs(""))
	p.RegisterOIDCProviderHandlers(root)

	limitingHandler := utils.APIBodyLimitingHandler(apiLimit)
	root.PathPrefix("/v1-saml").Handler(limitingHandler(saml))
	if features.V3Public.Enabled() {
		root.PathPrefix("/v3-public").Handler(limitingHandler(v3PublicAPI)) // Deprecated. Use /v1-public instead.
	}
	root.PathPrefix("/v1-public").Handler(limitingHandler(v1PublicAPI))
	root.NotFoundHandler = privateAPI

	return func(next http.Handler) http.Handler {
		privateAPI.NotFoundHandler = next
		return root
	}, nil
}

func newPrivateAPI(ctx context.Context, scaledContext *config.ScaledContext, authToken requests.AuthTokenGetter) (*mux.Router, error) {
	logout := logout.NewHandler(ctx, tokens.NewManager(scaledContext.Wrangler))

	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext.Wrangler, logout, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	otherAPIs, err := api.NewNormanServer(ctx, scaledContext, authToken)
	if err != nil {
		return nil, err
	}

	root := mux.NewRouter()
	root.UseEncodedPath()
	root.Use(requests.NewAuthenticatedFilter)
	root.PathPrefix("/v3/identit").Handler(tokenAPI)
	root.PathPrefix("/v3/token").Handler(tokenAPI)
	root.PathPrefix("/v3/authConfig").Handler(otherAPIs)
	root.PathPrefix("/v3/principal").Handler(otherAPIs)
	root.PathPrefix("/v3/user").Handler(otherAPIs)
	root.PathPrefix("/v3/schema").Handler(otherAPIs)
	root.PathPrefix("/v3/subscribe").Handler(otherAPIs)
	return root, nil
}

func (s *Server) OnLeader(ctx context.Context) error {
	if s.scaledContext == nil {
		return nil
	}

	management := &config.ManagementContext{
		Management: s.scaledContext.Management,
		Core:       s.scaledContext.Core,
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
		if features.Auth.Enabled() {
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
		} else {
			rw.Header().Set("X-API-Cattle-Auth", "none")
		}
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
