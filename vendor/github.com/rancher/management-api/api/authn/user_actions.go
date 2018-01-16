package authn

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/client/management/v3"
)

type ChangePasswordInput struct {
	NewPassword string
}

func UserFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "changepassword")
}

func UserActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return err
	}

	store := request.Schema.Store
	if store == nil {
		return errors.New("no user store available")
	}

	userData, err := store.ByID(request, request.Schema, request.ID)
	if err != nil {
		return err
	}

	newPass, ok := actionInput["newPassword"].(string)
	if !ok || len(newPass) == 0 {
		return errors.New("Invalid password")
	}

	userData[client.UserFieldPassword] = newPass
	if err := hashPassword(userData); err != nil {
		return err
	}
	userData[client.UserFieldMustChangePassword] = false

	userData, err = store.Update(request, request.Schema, userData, request.ID)
	if err != nil {
		return err
	}

	request.WriteResponse(http.StatusOK, userData)
	return nil
}
