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
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ap *azureProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
	resource.AddAction(apiContext, "upgrade")
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
		return ap.ConfigureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return ap.testAndApply(actionName, action, request)
	} else if actionName == "upgrade" {
		if err := ap.checkConfigurationBeforeMigration(); err != nil {
			return err
		}
		return ap.migrateToMicrosoftGraph()
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (ap *azureProvider) ConfigureTest(actionName string, action *types.Action, request *types.APIContext) error {
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
	azureADConfigApplyInput := &v32.AzureADConfigApplyInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(azureADConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	azureADConfig := &azureADConfigApplyInput.Config

	// This covers the case where users upgrade Rancher to v2.6.7+ without having used Azure AD as the auth provider.
	// In 2.6.7+, whether Azure AD is later registered or not, Rancher on startup creates the annotation on the template auth config.
	// But in the case where the auth config had been created on Rancher startup prior to v2.6.7, the annotation would be missing.
	// This ensures the annotation is set on next auth provider setup attempt.
	if azureADConfig.ObjectMeta.Annotations == nil {
		azureADConfig.ObjectMeta.Annotations = make(map[string]string)
	}
	azureADConfig.ObjectMeta.Annotations[GraphEndpointMigratedAnnotation] = "true"

	azureLogin := &v32.AzureADLogin{
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
	userPrincipal, groupPrincipals, providerToken, err := ap.loginUser(azureADConfig, azureLogin, true)
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

	err = ap.saveAzureConfigK8s(azureADConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save azure config: %v", err))
	}

	userExtraInfo := ap.GetUserExtraAttributes(userPrincipal)

	return ap.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via Azure Configuration", request, userExtraInfo)
}

// checkConfigurationBeforeMigration verifies that admins have properly configured an existing app registration's permissions
// in the Azure portal before they update their Azure AD auth config to use the new authentication flow.
// The method fetches the current AuthConfig from the database, then updates it in-memory to use the new endpoints,
// and creates a new test Azure client, thereby getting an access token to the Graph API.
// Then it parses the JWT and inspects the permissions contained within. If the admins had not set those as per docs,
// then Rancher won't find the permissions in the test token and will return an error.
func (ap *azureProvider) checkConfigurationBeforeMigration() error {
	cfg, err := ap.getAzureConfigK8s()
	if err != nil {
		return err
	}
	// Use the future, post-migration, endpoints. This temporarily updated config won't be persisted to the database in this method.
	updateAzureADEndpoints(cfg)

	azureClient, err := clients.NewMSGraphClient(cfg, ap.secrets)
	if err != nil {
		return err
	}
	token := azureClient.AccessToken()
	return clients.EnsureMSGraphTokenHasPermissions(token)
}

// migrateToMicrosoftGraph represents the migration of the registered Azure AD auth provider
// from the deprecated Azure AD Graph API to the Microsoft Graph API.
// It modifies the existing auth config value in the database, so that it has up-to-date endpoints to the new API.
// Most importantly, it sets the annotation that specifies that the auth config has been migrated to use the new auth flow.
func (ap *azureProvider) migrateToMicrosoftGraph() error {
	defer ap.deleteUserAccessTokens()
	defer clients.GroupCache.Purge()

	cfg, err := ap.getAzureConfigK8s()
	if err != nil {
		return err
	}
	if authProviderEnabled(cfg) {
		updateAzureADEndpoints(cfg)

		if cfg.ObjectMeta.Annotations == nil {
			cfg.ObjectMeta.Annotations = make(map[string]string)
		}
		cfg.ObjectMeta.Annotations[GraphEndpointMigratedAnnotation] = "true"

		err = ap.saveAzureConfigK8s(cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

// deleteUserAccessTokens attempts to delete all secrets that contain users' access tokens used for working with
// the deprecated Azure AD Graph API.
// It is not possible to filter secrets easily by presence of specific key(s) in the data object. The method fetches all
// Opaque secrets in the relevant namespace and looks at the target key in the data to find a secret that stores a user's
// access token to delete.
func (ap *azureProvider) deleteUserAccessTokens() {
	secrets, err := ap.secrets.ListNamespaced(tokens.SecretNamespace, metav1.ListOptions{FieldSelector: "type=Opaque"})
	if err != nil {
		logrus.Errorf("failed to fetch secrets: %v", err)
		return
	}
	// Provider name for Azure AD is the main key on secret data. This allows to identify the secrets to be deleted.
	const key = Name
	for _, secret := range secrets.Items {
		if _, keyPresent := secret.Data[key]; keyPresent {
			err := ap.secrets.DeleteNamespaced(tokens.SecretNamespace, secret.Name, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("failed to delete secret %s:%s - %v", tokens.SecretNamespace, secret.Name, err)
			}
		}
	}
}
