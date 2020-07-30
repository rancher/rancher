package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/providers/common"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/ref"
	mv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	mclient "github.com/rancher/types/client/management/v3"
	client "github.com/rancher/types/client/project/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	githubDefaultHostName = "https://github.com"
	actionDisable         = "disable"
	actionTestAndApply    = "testAndApply"
	actionLogin           = "login"
)

func (g *GhProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["enabled"]) {
		resource.AddAction(apiContext, actionDisable)
	}

	resource.AddAction(apiContext, actionTestAndApply)
}

func (g *GhProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionTestAndApply {
		return g.testAndApply(actionName, action, request)
	} else if actionName == actionDisable {
		return g.DisableAction(request, g.GetName())
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GhProvider) providerFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionLogin)
}

func (g *GhProvider) providerActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionLogin {
		return g.authuser(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func formGithubRedirectURLFromMap(config map[string]interface{}) string {
	hostname := convert.ToString(config[mclient.GithubConfigFieldHostname])
	clientID := convert.ToString(config[mclient.GithubConfigFieldClientID])
	tls := convert.ToBool(config[mclient.GithubConfigFieldTLS])
	return githubRedirectURL(hostname, clientID, tls)
}

func githubRedirectURL(hostname, clientID string, tls bool) string {
	redirect := ""
	if hostname != "" {
		scheme := "http://"
		if tls {
			scheme = "https://"
		}
		redirect = scheme + hostname
	} else {
		redirect = githubDefaultHostName
	}
	return fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&scope=repo+admin:repo_hook", redirect, clientID)
}

func (g *GhProvider) testAndApply(actionName string, action *types.Action, apiContext *types.APIContext) error {
	applyInput := &v3.GithubApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := g.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedGithubPipelineConfig, ok := pConfig.(*v3.GithubPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get github provider config")
	}
	toUpdate := storedGithubPipelineConfig.DeepCopy()

	if applyInput.InheritAuth {
		globalConfig, err := g.getGithubConfigCR()
		if err != nil {
			return err
		}
		toUpdate.Inherit = true
		toUpdate.ClientID = globalConfig.ClientID
		toUpdate.ClientSecret = globalConfig.ClientSecret
		toUpdate.Hostname = globalConfig.Hostname
		toUpdate.TLS = globalConfig.TLS
	} else {
		toUpdate.ClientID = applyInput.ClientID
		toUpdate.ClientSecret = applyInput.ClientSecret
		toUpdate.Hostname = applyInput.Hostname
		toUpdate.TLS = applyInput.TLS
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	sourceCodeCredential, err := g.AuthAddAccount(userName, applyInput.Code, toUpdate, toUpdate.ProjectName, model.GithubType)
	if err != nil {
		return err
	}
	if _, err = g.RefreshReposByCredentialAndConfig(sourceCodeCredential, toUpdate); err != nil {
		return err
	}

	toUpdate.Enabled = true
	//when inherit from global auth, we don't store client secret to project scope
	if toUpdate.Inherit {
		toUpdate.ClientSecret = ""
	}
	//update github pipeline config
	if _, err = g.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, nil)
	return nil
}

func (g *GhProvider) authuser(apiContext *types.APIContext) error {
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
	config, ok := pConfig.(*v3.GithubPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get github provider config")
	}
	if !config.Enabled {
		return errors.New("github oauth app is not configured")
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	account, err := g.AuthAddAccount(userName, authUserInput.Code, config, config.ProjectName, model.GithubType)
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

func (g *GhProvider) getGithubConfigCR() (*mv3.GithubConfig, error) {
	authConfigObj, err := g.AuthConfigs.ObjectClient().UnstructuredClient().Get(model.GithubType, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, cannot read k8s Unstructured data")
	}
	storedGithubConfigMap := u.UnstructuredContent()

	storedGithubConfig := &mv3.GithubConfig{}
	if err := mapstructure.Decode(storedGithubConfigMap, storedGithubConfig); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}

	objectMeta, err := common.ObjectMetaFromUnstructureContent(storedGithubConfigMap)
	if err != nil {
		return nil, err
	}
	storedGithubConfig.ObjectMeta = *objectMeta

	return storedGithubConfig, nil
}
