package keycloakoidc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
)

func (k *keyCloakOIDCProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (k *keyCloakOIDCProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, k.Name, k.AuthConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "configureTest" {
		return k.ConfigureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return k.TestAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (k *keyCloakOIDCProvider) ConfigureTest(actionName string, action *types.Action, request *types.APIContext) error {
	//verify body has all required fields
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		k.Type))
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"redirectUrl": k.GetRedirectURL(input),
		"type":        "OIDCTestOutput",
	}
	request.WriteResponse(http.StatusOK, data)
	return nil
}

func (k *keyCloakOIDCProvider) TestAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	var oidcConfig v32.OIDCConfig
	oidcConfigApplyInput := &v32.OIDCApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(oidcConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[generic oidc] testAndApply: Failed to parse body: %v", err))
	}

	oidcConfig = oidcConfigApplyInput.OIDCConfig
	oidcLogin := &v32.OIDCLogin{
		Code: oidcConfigApplyInput.Code,
	}

	//encode url to ensure path is escaped properly
	//the issuer url is used to get all the other urls for the provider
	//so its the only one that needs encoded
	issuerURL, err := url.Parse(oidcConfig.Issuer)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[generic oidc]: server error while authenticating")
	}
	oidcConfig.Issuer = issuerURL.String()

	//call provider
	userPrincipal, groupPrincipals, providerToken, err := k.LoginUser(request.Request.Context(), oidcLogin, &oidcConfig)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[generic oidc]: server error while authenticating")
	}

	user, err := k.UserMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	err = k.SaveOIDCConfig(&oidcConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save azure config: %v", err))
	}

	return k.TokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via OIDC Configuration", request)
}
