package googleoauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func (g *googleOauthProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (g *googleOauthProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, Name, g.authConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "configureTest" {
		return g.configureTest(request)
	} else if actionName == "testAndApply" {
		return g.testAndApply(request)
	}
	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *googleOauthProvider) configureTest(request *types.APIContext) error {
	goauthConfig := &v32.GoogleOauthConfig{}
	if err := json.NewDecoder(request.Request.Body).Decode(goauthConfig); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[Google OAuth] configureTest: Failed to parse body: %v", err))
	}

	redirectURL, err := g.formGoogleOAuthRedirectURL(goauthConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[Google OAuth] configureTest: Failed to form redirect URL with error: %v", err))
	}
	data := map[string]interface{}{
		"redirectUrl": redirectURL,
		"type":        "googleOAuthConfigTestOutput",
	}
	request.WriteResponse(http.StatusOK, data)
	return nil
}

func (g *googleOauthProvider) testAndApply(request *types.APIContext) error {
	var googleOAuthConfig v32.GoogleOauthConfig
	googleOAuthConfigApplyInput := &v32.GoogleOauthConfigApplyInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(googleOAuthConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[Google OAuth] testAndApply: Failed to parse body: %v", err))
	}

	googleOAuthConfig = googleOAuthConfigApplyInput.GoogleOauthConfig
	googleLogin := &v32.GoogleOauthLogin{
		Code: googleOAuthConfigApplyInput.Code,
	}

	if googleOAuthConfig.OauthCredential != "" {
		value, err := common.ReadFromSecret(g.secrets, googleOAuthConfig.OauthCredential,
			strings.ToLower(client.GoogleOauthConfigFieldOauthCredential))
		if err != nil {
			return err
		}
		googleOAuthConfig.OauthCredential = value
	}

	if googleOAuthConfig.ServiceAccountCredential != "" {
		value, err := common.ReadFromSecret(g.secrets, googleOAuthConfig.ServiceAccountCredential,
			strings.ToLower(client.GoogleOauthConfigFieldServiceAccountCredential))
		if err != nil {
			return err
		}
		googleOAuthConfig.ServiceAccountCredential = value
	}

	// Call provider to testLogin
	userPrincipal, groupPrincipals, providerInfo, err := g.loginUser(request.Request.Context(), googleLogin, &googleOAuthConfig, true)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return fmt.Errorf("[Google OAuth] testAndApply: server error while authenticating: %v", err)
	}
	// if this works, save google oauth CR adding enabled flag
	user, err := g.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	googleOAuthConfig.Enabled = googleOAuthConfigApplyInput.Enabled
	err = g.saveGoogleOAuthConfigCR(&googleOAuthConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("testAndApply: Failed to save google oauth config: %v", err))
	}

	userExtraInfo := g.GetUserExtraAttributes(userPrincipal)
	err = g.tokenMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("testAndApply: Failed to create or update userAttribute: %v", err))
	}

	return g.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via Google OAuth Configuration", request)

}

func (g *googleOauthProvider) formGoogleOAuthRedirectURL(goauthConfig *v32.GoogleOauthConfig) (string, error) {
	return g.getRedirectURL([]byte(goauthConfig.OauthCredential))
}

func (g *googleOauthProvider) formGoogleOAuthRedirectURLFromMap(config map[string]interface{}) (string, error) {
	clientCreds, ok := config[client.GoogleOauthConfigFieldOauthCredential].(string)
	if !ok {
		return "", fmt.Errorf("[Google OAuth] formGoogleOAuthRedirectURLFromMap: no creds file present")
	}
	value, err := common.ReadFromSecret(g.secrets, clientCreds, strings.ToLower(client.GoogleOauthConfigFieldOauthCredential))
	if err != nil {
		return "", err
	}

	return g.getRedirectURL([]byte(value))
}

func (g *googleOauthProvider) getRedirectURL(configFile []byte) (string, error) {
	oauth2Config, err := google.ConfigFromJSON(configFile)
	if err != nil {
		return "", err
	}
	// Removing redirectURL from config because UI will set it
	oauth2Config.RedirectURL = ""
	// access type=offline and prompt=consent (approval force), return a refresh token
	// UI will generate and validate the state
	consentPageURL := oauth2Config.AuthCodeURL("", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return consentPageURL, nil
}
