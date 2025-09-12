package oidc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"k8s.io/client-go/util/retry"
)

const cognitoGroupsClaim = "cognito:groups"

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

// TestAndApply validates the correctness of the OIDC configuration
// provided in the request.
// If the verification succeed, creates a Token to access the provider.
// It returns an error in case of failure.
func (o *OpenIDCProvider) TestAndApply(request *types.APIContext) error {
	var oidcConfig v32.OIDCConfig
	oidcConfigApplyInput := &v32.OIDCApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(oidcConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[generic oidc] testAndApply: failed to parse body: %v", err))
	}

	oidcConfig = oidcConfigApplyInput.OIDCConfig
	// set a default value for GroupSearchEnabled
	// in case user input is nil for some reasons.
	if oidcConfigApplyInput.OIDCConfig.GroupSearchEnabled == nil {
		falseBool := false
		oidcConfig.GroupSearchEnabled = &falseBool
	}
	// we need to set cognito:groups as GroupsClaim in order to be able to fetch groups from aws cognito
	if oidcConfig.Type == client.CognitoConfigType {
		oidcConfig.GroupsClaim = cognitoGroupsClaim
	}
	oidcLogin := &v32.OIDCLogin{
		Code: oidcConfigApplyInput.Code,
	}

	if !validateScopes(oidcConfig.Scopes) {
		return fmt.Errorf("scopes are invalid:  scopes must be space delimited and openid is a required scope. %s", oidcConfig.Scopes)
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
	userPrincipal, groupPrincipals, providerToken, _, err := o.LoginUser(request.Request.Context(), oidcLogin, &oidcConfig)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[generic oidc]: server error while authenticating")
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
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return o.TokenMgr.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	}); err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[generic oidc]: Failed to create or update userAttribute: %v", err))
	}

	return o.TokenMgr.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via OIDC Configuration", request)
}

// validateScopes returns true if there are no commas in the scopes string and openid is included as a scope.
// Otherwise, the scopes are invalid and we return false.
func validateScopes(input string) bool {
	if strings.Contains(input, ",") {
		return false
	}
	values := strings.Fields(input)
	return slices.Contains(values, "openid")
}
