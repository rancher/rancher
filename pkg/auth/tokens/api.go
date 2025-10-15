package tokens

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

const (
	CookieName      = "R_SESS"
	AuthHeaderName  = "Authorization"
	AuthValuePrefix = "Bearer"
	BasicAuthPrefix = "Basic"
	CSRFCookie      = "CSRF"
)

type ServerOption func(server *normanapi.Server)

func NewAPIHandler(ctx context.Context, wContext *wrangler.Context, logout http.Handler, opts ...ServerOption) (http.Handler, error) {
	tokenMgr := NewManager(wContext)
	api := &tokenAPI{
		mgr:           tokenMgr,
		logoutHandler: logout,
	}

	schemas := types.NewSchemas().AddSchemas(managementSchema.TokenSchemas)
	schema := schemas.Schema(&managementSchema.Version, client.TokenType)
	schema.CollectionActions = map[string]types.Action{
		"logout":    {},
		"logoutAll": {},
	}

	schema.ActionHandler = api.tokenActionHandler
	schema.ListHandler = api.tokenListHandler
	schema.CreateHandler = api.tokenCreateHandler
	schema.DeleteHandler = api.tokenDeleteHandler

	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(server)
	}

	return server, nil
}

type tokenAPI struct {
	mgr           *Manager
	logoutHandler http.Handler
}

func (t *tokenAPI) tokenActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	logrus.Debugf("TokenActionHandler called for action %v", actionName)
	if actionName == "logout" || actionName == "logoutAll" {
		t.logoutHandler.ServeHTTP(request.Response, request.Request)
		return nil
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (t *tokenAPI) tokenCreateHandler(request *types.APIContext, _ types.RequestHandler) error {
	logrus.Debugf("TokenCreateHandler called")
	return t.mgr.deriveToken(request)
}

func (t *tokenAPI) tokenListHandler(request *types.APIContext, _ types.RequestHandler) error {
	logrus.Debugf("TokenListHandler called")
	if request.ID != "" {
		return t.mgr.getTokenFromRequest(request)
	}
	return t.mgr.listTokens(request)
}

func (t *tokenAPI) tokenDeleteHandler(request *types.APIContext, _ types.RequestHandler) error {
	logrus.Debugf("TokenDeleteHandler called")
	return t.mgr.removeToken(request)
}
