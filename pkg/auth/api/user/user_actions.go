package user

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"golang.org/x/crypto/bcrypt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) UserFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "setpassword")

	if canRefresh := h.userCanRefresh(apiContext); canRefresh {
		resource.AddAction(apiContext, "refreshauthprovideraccess")
	}
}

func (h *Handler) CollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, "changepassword")
	if canRefresh := h.userCanRefresh(apiContext); canRefresh {
		collection.AddAction(apiContext, "refreshauthprovideraccess")
	}
}

type Handler struct {
	UserClient               v3.UserInterface
	GlobalRoleBindingsClient v3.GlobalRoleBindingInterface
	UserAuthRefresher        providerrefresh.UserAuthRefresher
}

func (h *Handler) Actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "changepassword":
		if err := h.changePassword(actionName, action, apiContext); err != nil {
			return err
		}
	case "setpassword":
		if err := h.setPassword(actionName, action, apiContext); err != nil {
			return err
		}
	case "refreshauthprovideraccess":
		if err := h.refreshAttributes(actionName, action, apiContext); err != nil {
			return err
		}
	default:
		return errors.Errorf("bad action %v", actionName)
	}

	if !strings.EqualFold(settings.FirstLogin.Get(), "false") {
		if err := settings.FirstLogin.Set("false"); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) changePassword(actionName string, action *types.Action, request *types.APIContext) error {
	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return err
	}

	store := request.Schema.Store
	if store == nil {
		return errors.New("no user store available")
	}

	userID := request.Request.Header.Get("Impersonate-User")
	if userID == "" {
		return errors.New("can't find user")
	}

	currentPass, ok := actionInput["currentPassword"].(string)
	if !ok || len(currentPass) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must specify current password")
	}

	newPass, ok := actionInput["newPassword"].(string)
	if !ok || len(newPass) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid new password")
	}

	user, err := h.UserClient.Get(userID, v1.GetOptions{})
	if err != nil {
		return err
	}

	if err := validatePassword(user.Username, currentPass, newPass, settings.PasswordMinLength.GetInt()); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPass)); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid current password")
	}

	newPassHash, err := HashPasswordString(newPass)
	if err != nil {
		return err
	}

	user.Password = newPassHash
	user.MustChangePassword = false
	user, err = h.UserClient.Update(user)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) setPassword(actionName string, action *types.Action, request *types.APIContext) error {
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

	username := userData[client.UserFieldUsername].(string)

	// passing empty currentPass to validator since, this api call doesn't assume an existing password
	if err := validatePassword(username, "", newPass, settings.PasswordMinLength.GetInt()); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	userData[client.UserFieldPassword] = newPass
	if err := hashPassword(userData); err != nil {
		return err
	}
	userData[client.UserFieldMustChangePassword] = false
	delete(userData, "me")

	userData, err = store.Update(request, request.Schema, userData, request.ID)
	if err != nil {
		return err
	}

	request.WriteResponse(http.StatusOK, userData)
	return nil
}

func (h *Handler) refreshAttributes(actionName string, action *types.Action, request *types.APIContext) error {
	canRefresh := h.userCanRefresh(request)

	if !canRefresh {
		return errors.New("Not Allowed")
	}

	if request.ID != "" {
		h.UserAuthRefresher.TriggerUserRefresh(request.ID, true)
	} else {
		h.UserAuthRefresher.TriggerAllUserRefresh()
	}

	request.WriteResponse(http.StatusOK, nil)
	return nil
}

func (h *Handler) userCanRefresh(request *types.APIContext) bool {
	return request.AccessControl.CanDo(v3.UserGroupVersionKind.Group, v3.UserResource.Name, "create", request, nil, request.Schema) == nil
}

// validatePassword will ensure a password is at least the minimum required length in runes,
// that the username and password do not match, and that the new password is not the same as the current password.
func validatePassword(user string, currentPass string, pass string, minPassLen int) error {
	if utf8.RuneCountInString(pass) < minPassLen {
		return errors.Errorf("Password must be at least %v characters", minPassLen)
	}

	if user == pass {
		return errors.New("Password cannot be the same as username")
	}
	if pass == currentPass {
		return errors.New("The new password must not be the same as the current password")
	}

	return nil
}
