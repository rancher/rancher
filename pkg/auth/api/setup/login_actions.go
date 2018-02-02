package setup

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/tokens"
)

func AuthProviderFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "login")
}

func ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == "login" {
		return tokens.Login(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}
