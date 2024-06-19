package oidc

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

func (o *OpenIDCProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (o *OpenIDCProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, o.Name, o.AuthConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "configureTest" {
		return o.ConfigureTest(request)
	} else if actionName == "testAndApply" {
		return o.TestAndApply(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (o *OpenIDCProvider) ConfigureTest(request *types.APIContext) error {
	//verify body has all required fields
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		o.Type))
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"redirectUrl": o.getRedirectURL(input),
		"type":        "OIDCTestOutput",
	}
	request.WriteResponse(http.StatusOK, data)
	return nil
}

func (o *OpenIDCProvider) TestAndApply(request *types.APIContext) error {
	var oidcConfig v32.OIDCConfig
	oidcConfigApplyInput := &v32.OIDCApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(oidcConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[generic oidc] testAndApply: failed to parse body: %v", err))
	}

	oidcConfig = oidcConfigApplyInput.OIDCConfig
	oidcLogin := &v32.OIDCLogin{
		Code: oidcConfigApplyInput.Code,
	}

	// encode url to ensure path is escaped properly
	// the issuer url is used to get all the other urls for the provider
	// so its the only one that needs encoded
	issuerURL, err := url.Parse(oidcConfig.Issuer)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[generic oidc]: server error while authenticating")
	}
	oidcConfig.Issuer = issuerURL.String()

	// call provider
	userPrincipal, groupPrincipals, providerToken, claimInfo, err := o.LoginUser(request.Request.Context(), oidcLogin, &oidcConfig)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[generic oidc]: server error while authenticating")
	}
	// setting a bool for group search flag
	// this only needs updated when an auth provider is enabled or edited
	if claimInfo.Groups == nil && claimInfo.FullGroupPath == nil {
		falseBool := false
		oidcConfig.GroupSearchEnabled = &falseBool
	} else {
		trueBool := true
		oidcConfig.GroupSearchEnabled = &trueBool
	}
	user, err := o.UserMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	err = o.saveOIDCConfig(&oidcConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[generic oidc]: failed to save oidc config: %v", err))
	}

	userExtraInfo := o.GetUserExtraAttributes(userPrincipal)
	err = o.TokenMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[generic oidc]: Failed to create or update userAttribute: %v", err))
	}

	return o.TokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via OIDC Configuration", request)
}
