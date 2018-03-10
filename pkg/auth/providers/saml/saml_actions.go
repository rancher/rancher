package saml

import (
	"encoding/json"
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (s *Provider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "testAndEnable")
}

func (s *Provider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, s.name, s.authConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "testAndEnable" {
		return s.testAndEnable(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (s *Provider) testAndEnable(actionName string, action *types.Action, request *types.APIContext) error {
	// get Final redirect URL from request body

	samlLogin := &v3.SamlConfigTestInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(samlLogin); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("SAML: Failed to parse body: %v", err))
	}

	samlConfig, err := s.getSamlConfig()
	if err != nil {
		return err
	}

	err = InitializeSamlServiceProvider(samlConfig, s.name)
	if err != nil {
		return err
	}

	provider := SamlProviders[s.name]

	finalRedirectURL := samlLogin.FinalRedirectURL
	provider.clientState.SetState(request.Response, request.Request, "Rancher_UserID", provider.userMGR.GetUser(request))
	provider.clientState.SetState(request.Response, request.Request, "Rancher_FinalRedirectURL", finalRedirectURL)
	provider.clientState.SetState(request.Response, request.Request, "Rancher_Action", "testAndEnable")
	provider.HandleSamlLogin(request.Response, request.Request)
	return nil
}

func (s *Provider) formSamlRedirectURL(samlConfig *v3.SamlConfig) string {
	var path string
	if s.name == PingName {
		path = samlConfig.RancherAPIHost + "/v1-saml/" + PingName + "/login"
	}

	return path
}
