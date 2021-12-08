package saml

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/sirupsen/logrus"
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

	samlLogin := &v32.SamlConfigTestInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(samlLogin); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("SAML: Failed to parse body: %v", err))
	}

	samlConfig, err := s.getSamlConfig()
	if err != nil {
		return err
	}

	logrus.Debug("SAML [testAndEnable]: Initializing SAML service provider")
	err = InitializeSamlServiceProvider(samlConfig, s.name)
	if err != nil {
		return err
	}

	provider, ok := SamlProviders[s.name]
	if !ok {
		return fmt.Errorf("SAML [testAndEnable]: Provider %v not configured", s.name)
	}

	logrus.Debugf("SAML [testAndEnable]: Setting clientState for SAML service provider %v", s.name)
	finalRedirectURL := samlLogin.FinalRedirectURL
	provider.clientState.SetState(request.Response, request.Request, "Rancher_UserID", provider.userMGR.GetUser(request))
	provider.clientState.SetState(request.Response, request.Request, "Rancher_FinalRedirectURL", finalRedirectURL)
	provider.clientState.SetState(request.Response, request.Request, "Rancher_Action", testAndEnableAction)
	idpRedirectURL, err := provider.HandleSamlLogin(request.Response, request.Request)
	if err != nil {
		return err
	}
	logrus.Debugf("SAML [testAndEnable]: Redirecting to the identity provider login page at %v", idpRedirectURL)
	data := map[string]interface{}{
		"idpRedirectUrl": idpRedirectURL,
		"type":           "samlConfigTestOutput",
	}

	request.WriteResponse(http.StatusOK, data)
	return nil
}
