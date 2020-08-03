package principals

import (
	"context"
	"encoding/json"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"net/http"

	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

type principalsHandler struct {
	principalsClient v3.PrincipalInterface
	tokensClient     v3.TokenInterface
	auth             requests.Authenticator
	tokenMGR         *tokens.Manager
	ac               types.AccessControl
}

func newPrincipalsHandler(ctx context.Context, clusterRouter requests.ClusterRouter, mgmt *config.ScaledContext) *principalsHandler {
	providers.Configure(ctx, mgmt)
	return &principalsHandler{
		principalsClient: mgmt.Management.Principals(""),
		tokensClient:     mgmt.Management.Tokens(""),
		auth:             requests.NewAuthenticator(ctx, clusterRouter, mgmt),
		tokenMGR:         tokens.NewManager(ctx, mgmt),
		ac:               mgmt.AccessControl,
	}
}

func (h *principalsHandler) actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "search" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	input := &v32.SearchPrincipalsInput{}
	if err := json.NewDecoder(apiContext.Request.Body).Decode(input); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("Failed to parse body: %v", err))
	}

	token, err := h.getToken(apiContext.Request)
	if err != nil {
		return err
	}

	ps, err := providers.SearchPrincipals(input.Name, input.PrincipalType, *token)
	if err != nil {
		return err
	}

	var principals []map[string]interface{}
	for _, p := range ps {
		x, err := convertPrincipal(apiContext.Schema, p)
		if err != nil {
			return err
		}
		principals = append(principals, x)
	}

	context := map[string]string{"resource": "principals", "apiGroup": "management.cattle.io"}
	principals = h.ac.FilterList(apiContext, apiContext.Schema, principals, context)

	apiContext.WriteResponse(200, principals)
	return nil
}

func (h *principalsHandler) list(apiContext *types.APIContext, next types.RequestHandler) error {
	var principals []map[string]interface{}

	token, err := h.getToken(apiContext.Request)
	if err != nil {
		return err
	}

	if apiContext.ID != "" {
		princ, err := providers.GetPrincipal(apiContext.ID, *token)
		if err != nil {
			return err
		}
		p, err := convertPrincipal(apiContext.Schema, princ)
		if err != nil {
			return err
		}

		context := map[string]string{"resource": "principals", "apiGroup": "management.cattle.io"}
		p = h.ac.Filter(apiContext, apiContext.Schema, p, context)

		apiContext.WriteResponse(200, p)
		return nil
	}

	p, err := convertPrincipal(apiContext.Schema, token.UserPrincipal)
	if err != nil {
		return err
	}
	principals = append(principals, p)

	groupPrincipals := h.tokenMGR.GetGroupsForTokenAuthProvider(token)
	for _, p := range groupPrincipals {
		x, err := convertPrincipal(apiContext.Schema, p)
		if err != nil {
			return err
		}
		principals = append(principals, x)
	}

	apiContext.WriteResponse(200, principals)
	return nil
}

func convertPrincipal(schema *types.Schema, principal v3.Principal) (map[string]interface{}, error) {
	data, err := convert.EncodeToMap(principal)
	if err != nil {
		return nil, err
	}
	mapper := schema.Mapper
	if mapper == nil {
		return nil, errors.New("no schema mapper available")
	}
	mapper.FromInternal(data)

	return data, nil
}

func (h *principalsHandler) getToken(request *http.Request) (*v3.Token, error) {
	token, err := h.auth.TokenFromRequest(request)
	return token, errors.Wrap(err, requests.ErrMustAuthenticate.Error())
}
