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
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PasswordUpdater interface {
	VerifyAndUpdatePassword(userId string, currentPassword, newPassword string) error
	UpdatePassword(userId string, newPassword string) error
}

func (h *Handler) UserFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "setpassword")

	if canRefresh := h.userCanRefresh(apiContext); canRefresh {
		resource.AddAction(apiContext, "refreshauthprovideraccess")
	}

	if !h.canDoUserAction(apiContext, "update") {
		delete(resource.Links, "update")
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
	ExtTokenStore            *exttokenstore.SystemStore
	SecretLister             wranglerv1.SecretCache
	SecretClient             wranglerv1.SecretClient
	PwdChanger               PasswordUpdater
}

func (h *Handler) Actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "changepassword":
		if err := h.changePassword(apiContext); err != nil {
			return err
		}
	case "setpassword":
		if err := h.setPassword(apiContext); err != nil {
			return err
		}
	case "refreshauthprovideraccess":
		if err := h.refreshAttributes(apiContext); err != nil {
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

func (h *Handler) changePassword(request *types.APIContext) error {
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

	if err := h.PwdChanger.VerifyAndUpdatePassword(user.Name, currentPass, newPass); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	user.MustChangePassword = false
	_, err = h.UserClient.Update(user)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) setPassword(request *types.APIContext) error {
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

	// if the username is not set the user is an external one
	usernameInt, found := userData[client.UserFieldUsername]
	if !found {
		return errors.New("Cannot set password of non-local user")
	}
	username, _ := usernameInt.(string)

	// passing empty currentPass to validator since, this api call doesn't assume an existing password
	if err := validatePassword(username, "", newPass, settings.PasswordMinLength.GetInt()); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	userId, ok := userData[types.ResourceFieldID].(string)
	if !ok {
		return errors.New("failed to get userId")
	}
	if err := h.PwdChanger.UpdatePassword(userId, newPass); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
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

func (h *Handler) refreshAttributes(request *types.APIContext) error {
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
	return h.canDoUserAction(request, "create")
}

// canDoUserAction checks if the user has permission to perform the specified action on user resources.
func (h *Handler) canDoUserAction(apiContext *types.APIContext, verb string) bool {
	return apiContext.AccessControl.CanDo(v3.UserGroupVersionKind.Group, v3.UserResource.Name, verb, apiContext, nil, apiContext.Schema) == nil
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
