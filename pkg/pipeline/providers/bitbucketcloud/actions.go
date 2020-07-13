package bitbucketcloud

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/types/client/project/v3"
)

const (
	bitbucketDefaultHostName = "https://bitbucket.org"
	actionDisable            = "disable"
	actionTestAndApply       = "testAndApply"
	actionLogin              = "login"
)

func (b *BcProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["enabled"]) {
		resource.AddAction(apiContext, actionDisable)
	}

	resource.AddAction(apiContext, actionTestAndApply)
}

func (b *BcProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionTestAndApply {
		return b.testAndApply(actionName, action, request)
	} else if actionName == actionDisable {
		return b.DisableAction(request, b.GetName())
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (b *BcProvider) providerFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionLogin)
}

func (b *BcProvider) providerActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionLogin {
		return b.authuser(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func formBitbucketRedirectURLFromMap(config map[string]interface{}) string {
	clientID := convert.ToString(config[client.BitbucketCloudPipelineConfigFieldClientID])
	return fmt.Sprintf("%s/site/oauth2/authorize?client_id=%s&response_type=code", bitbucketDefaultHostName, clientID)
}

func (b *BcProvider) testAndApply(actionName string, action *types.Action, apiContext *types.APIContext) error {
	applyInput := &v3.BitbucketCloudApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedBitbucketPipelineConfig, ok := pConfig.(*v3.BitbucketCloudPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get github provider config")
	}
	toUpdate := storedBitbucketPipelineConfig.DeepCopy()
	toUpdate.ClientID = applyInput.ClientID
	toUpdate.ClientSecret = applyInput.ClientSecret
	toUpdate.RedirectURL = applyInput.RedirectURL

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	sourceCodeCredential, err := b.AuthAddAccount(userName, applyInput.Code, toUpdate, toUpdate.ProjectName, model.BitbucketCloudType)
	if err != nil {
		return err
	}
	if _, err = b.RefreshReposByCredentialAndConfig(sourceCodeCredential, toUpdate); err != nil {
		return err
	}

	toUpdate.Enabled = true
	//update bitbucket pipeline config
	if _, err = b.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, nil)
	return nil
}

func (b *BcProvider) authuser(apiContext *types.APIContext) error {
	authUserInput := v3.AuthUserInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &authUserInput); err != nil {
		return err
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	config, ok := pConfig.(*v3.BitbucketCloudPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get bitbucket provider config")
	}
	if !config.Enabled {
		return errors.New("bitbucket oauth app is not configured")
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	account, err := b.AuthAddAccount(userName, authUserInput.Code, config, config.ProjectName, model.BitbucketCloudType)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.SourceCodeCredentialType, account.Name, &data); err != nil {
		return err
	}

	if _, err := b.RefreshReposByCredentialAndConfig(account, config); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}
