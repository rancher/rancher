package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	client "github.com/rancher/types/client/management/v3"
)

func (ap *azureProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (ap *azureProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, Name, ap.authConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "configureTest" {
		return ap.configureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return ap.testAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (ap *azureProvider) configureTest(actionName string, action *types.Action, request *types.APIContext) error {
	// Verify the body has all required fields
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		client.AzureADConfigType))
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"redirectUrl": formAzureRedirectURL(input),
		"type":        "azureADConfigTestOutput",
	}

	request.WriteResponse(http.StatusOK, data)
	return nil
}

func (ap *azureProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	azureADConfigApplyInput := &v3.AzureADConfigApplyInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(azureADConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	azureADConfig := azureADConfigApplyInput.Config
	azureLogin := &v3public.AzureADLogin{
		Code: azureADConfigApplyInput.Code,
	}

	if azureADConfig.ApplicationSecret != "" {
		value, err := common.ReadFromSecret(ap.secrets, azureADConfig.ApplicationSecret,
			strings.ToLower(client.AzureADConfigFieldApplicationSecret))
		if err != nil {
			return err
		}
		azureADConfig.ApplicationSecret = value
	}
	//Call provider
	userPrincipal, groupPrincipals, providerToken, err := ap.loginUser(azureLogin, &azureADConfig, true)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "server error while authenticating")
	}

	user, err := ap.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	err = ap.saveAzureConfigK8s(&azureADConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save azure config: %v", err))
	}

	return ap.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via Azure Configuration", request)
}
