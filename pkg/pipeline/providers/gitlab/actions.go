package gitlab

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

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
	gitlabDefaultHostName = "https://gitlab.com"
	actionDisable         = "disable"
	actionTestAndApply    = "testAndApply"
	actionLogin           = "login"
)

func (g *GlProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["enabled"]) {
		resource.AddAction(apiContext, actionDisable)
	}

	resource.AddAction(apiContext, actionTestAndApply)
}

func (g *GlProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionTestAndApply {
		return g.testAndApply(actionName, action, request)
	} else if actionName == actionDisable {
		return g.DisableAction(request, g.GetName())
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GlProvider) providerFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionLogin)
}

func (g *GlProvider) providerActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionLogin {
		return g.authuser(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GlProvider) testAndApply(actionName string, action *types.Action, apiContext *types.APIContext) error {
	applyInput := &v3.GitlabApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := g.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedGitlabPipelineConfig, ok := pConfig.(*v3.GitlabPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get gitlab provider config")
	}
	toUpdate := storedGitlabPipelineConfig.DeepCopy()

	toUpdate.ClientID = applyInput.ClientID
	toUpdate.ClientSecret = applyInput.ClientSecret
	toUpdate.Hostname = applyInput.Hostname
	toUpdate.TLS = applyInput.TLS
	currentURL := apiContext.URLBuilder.Current()
	u, err := url.Parse(currentURL)
	if err != nil {
		return err
	}
	toUpdate.RedirectURL = fmt.Sprintf("%s://%s/verify-auth", u.Scheme, u.Host)
	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	sourceCodeCredential, err := g.AuthAddAccount(userName, applyInput.Code, toUpdate, toUpdate.ProjectName, model.GitlabType)
	if err != nil {
		return err
	}
	if _, err = g.RefreshReposByCredentialAndConfig(sourceCodeCredential, toUpdate); err != nil {
		return err
	}
	toUpdate.Enabled = true
	//update gitlab pipeline config
	if _, err = g.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, nil)
	return nil
}

func (g *GlProvider) authuser(apiContext *types.APIContext) error {
	authUserInput := v3.AuthUserInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &authUserInput); err != nil {
		return err
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := g.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	config, ok := pConfig.(*v3.GitlabPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get gitlab provider config")
	}
	if !config.Enabled {
		return errors.New("gitlab oauth app is not configured")
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	account, err := g.AuthAddAccount(userName, authUserInput.Code, config, config.ProjectName, model.GitlabType)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.SourceCodeCredentialType, account.Name, &data); err != nil {
		return err
	}

	if _, err := g.RefreshReposByCredentialAndConfig(account, config); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}
