package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/rancher/pkg/api/norman"
	"github.com/rancher/rancher/pkg/auth/api"
	"github.com/rancher/rancher/pkg/auth/data"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/rancher/pkg/types/config"
	steveauth "github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type Server struct {
	Authenticator steveauth.Middleware
	Management    func(http.Handler) http.Handler
	scaledContext *config.ScaledContext
}

func NewServer(ctx context.Context, cfg *rest.Config) (*Server, error) {
	sc, err := config.NewScaledContext(*cfg, nil)
	if err != nil {
		return nil, err
	}

	sc.UserManager, err = common.NewUserManagerNoBindings(sc)
	if err != nil {
		return nil, err
	}

	sc.ClientGetter, err = proxy.NewClientGetterFromConfig(*cfg)
	if err != nil {
		return nil, err
	}

	authenticator := requests.NewAuthenticator(ctx, clusterrouter.GetClusterID, sc)
	authManagement, err := newAPIManagement(ctx, sc)
	if err != nil {
		return nil, err
	}

	return &Server{
		Authenticator: requests.ToAuthMiddleware(authenticator),
		Management:    authManagement,
		scaledContext: sc,
	}, nil
}

func newAPIManagement(ctx context.Context, scaledContext *config.ScaledContext) (steveauth.Middleware, error) {
	privateAPI, err := newPrivateAPI(ctx, scaledContext)
	if err != nil {
		return nil, err
	}

	publicAPI, err := publicapi.NewHandler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	saml := saml.AuthHandler()

	root := mux.NewRouter()
	root.UseEncodedPath()
	root.PathPrefix("/v3-public").Handler(publicAPI)
	root.PathPrefix("/v1-saml").Handler(saml)
	root.NotFoundHandler = privateAPI

	return func(next http.Handler) http.Handler {
		privateAPI.NotFoundHandler = next
		return root
	}, nil
}

func newPrivateAPI(ctx context.Context, scaledContext *config.ScaledContext) (*mux.Router, error) {
	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	otherAPIs, err := api.NewNormanServer(ctx, clusterrouter.GetClusterID, scaledContext)
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
	return root, nil
}

func (s *Server) OnLeader(ctx context.Context) error {
	management := &config.ManagementContext{
		Management: s.scaledContext.Management,
		Core:       s.scaledContext.Core,
	}

	if err := data.AuthConfigs(management); err != nil {
		return fmt.Errorf("failed to add authconfig data: %v", err)
	}

	tokens.StartPurgeDaemon(ctx, management)
	providerrefresh.StartRefreshDaemon(ctx, s.scaledContext, management)
	logrus.Infof("Steve auth startup complete")
	return nil
}

func (s *Server) Start(ctx context.Context, leader bool) error {
	if err := s.scaledContext.Start(ctx); err != nil {
		return err
	}
	if leader {
		return s.OnLeader(ctx)
	}
	return nil
}
