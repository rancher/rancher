package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/rancher/rancher/pkg/api/norman"
	"github.com/rancher/rancher/pkg/auth/api"
	"github.com/rancher/rancher/pkg/auth/data"
	"github.com/rancher/rancher/pkg/auth/handler"
	"github.com/rancher/rancher/pkg/auth/logout"
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

	apiLimit, err := quantityAsInt64(getEnvWithDefault("CATTLE_AUTH_API_BODY_LIMIT", "1Mi"), 1024*1024)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Configuring auth server API body limit to %v bytes", apiLimit)

	p := handler.NewFromAuthConfigInterface(scaledContext.Management.AuthConfigs(""))

	limitingHandler := utils.APIBodyLimitingHandler(apiLimit)

	return func(next http.Handler) http.Handler {
		root := http.NewServeMux()

		// OIDC provider routes
		p.RegisterOIDCProviderHandlers(root)

		// Public routes with rate limiting
		root.Handle("/v1-saml/", limitingHandler(saml))
		if features.V3Public.Enabled() {
			root.Handle("/v3-public/", limitingHandler(v3PublicAPI))
		}
		root.Handle("/v1-public/", limitingHandler(v1PublicAPI))
		if features.SCIM.Enabled() {
			root.Handle(scim.URLPrefix+"/", limitingHandler(scim.NewHandler(scaledContext)))
		}

		// Private API routes
		root.Handle("/v3/identit/", privateAPI)
		root.Handle("/v3/token/", privateAPI)
		root.Handle("/v3/authConfig/", privateAPI)
		root.Handle("/v3/principal/", privateAPI)
		root.Handle("/v3/user/", privateAPI)
		root.Handle("/v3/schema/", privateAPI)
		root.Handle("/v3/subscribe/", privateAPI)

		// Fallback to next handler
		root.Handle("/", next)

		return root
	}, nil
}

func newPrivateAPI(ctx context.Context, scaledContext *config.ScaledContext, authToken requests.AuthTokenGetter) (http.Handler, error) {
	logout := logout.NewHandler(ctx, tokens.NewManager(scaledContext.Wrangler))

	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext.Wrangler, logout, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	otherAPIs, err := api.NewNormanServer(ctx, scaledContext, authToken)
	if err != nil {
		return nil, err
	}

	root := http.NewServeMux()

	// Apply middleware by wrapping handlers
	authFilter := requests.NewAuthenticatedFilter

	root.Handle("/v3/identit/", authFilter(tokenAPI))
	root.Handle("/v3/token/", authFilter(tokenAPI))
	root.Handle("/v3/authConfig/", authFilter(otherAPIs))
	root.Handle("/v3/principal/", authFilter(otherAPIs))
	root.Handle("/v3/user/", authFilter(otherAPIs))
	root.Handle("/v3/schema/", authFilter(otherAPIs))
	root.Handle("/v3/subscribe/", authFilter(otherAPIs))

	return authFilter(root), nil
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

func (s *Server) OnLeader(ctx context.Context) error {
	if s.scaledContext == nil {
		return nil
	}

	management := &config.ManagementContext{
		Management: s.scaledContext.Management,
		Core:       s.scaledContext.Core,
	}

	return data.AuthConfigs(management)
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

func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func quantityAsInt64(quantityStr string, defaultValue int64) (int64, error) {
	quantity, err := resource.ParseQuantity(quantityStr)
	if err != nil {
		return 0, err
	}
	value, ok := quantity.AsInt64()
	if !ok {
		return 0, fmt.Errorf("quantity %s cannot be converted to int64", quantityStr)
	}
	return value, nil
}
