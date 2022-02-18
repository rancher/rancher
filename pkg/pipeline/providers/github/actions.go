package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	auth "github.com/rancher/rancher/pkg/auth/providers/common"
	mclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	client "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	applyInput := &v32.GithubApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := g.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedGithubPipelineConfig, ok := pConfig.(*v32.GithubPipelineConfig)
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

	var secret *corev1.Secret
	if !toUpdate.Inherit {
		// if auth is inherited, then there is already a secret
		clusterID, _ := ref.Parse(apiContext.SubContext["/v3/schemas/project"])
		cluster, err := g.ClusterLister.Get("", clusterID)
		if err != nil {
			return err
		}
		secret, err = g.SecretMigrator.CreateOrUpdateSourceCodeProviderConfigSecret("", toUpdate.ClientSecret, cluster, model.GithubType)
		if err != nil {
			return err
		}
		toUpdate.CredentialSecret = secret.Name
	}
	toUpdate.ClientSecret = ""
	//update github pipeline config
	if _, err = g.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		if secret != nil {
			if cleanupErr := g.SecretMigrator.Cleanup(secret.Name); cleanupErr != nil {
				logrus.Errorf("github pipeline: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		return err
	}

	apiContext.WriteResponse(http.StatusOK, nil)
	return nil
}

func (g *GhProvider) authuser(apiContext *types.APIContext) error {
	authUserInput := v32.AuthUserInput{}
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
	config, ok := pConfig.(*v32.GithubPipelineConfig)
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

func (g *GhProvider) getGithubConfigCR() (*v33.GithubConfig, error) {
	authConfigObj, err := g.AuthConfigs.ObjectClient().UnstructuredClient().Get(model.GithubType, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, cannot read k8s Unstructured data")
	}
	storedGithubConfigMap := u.UnstructuredContent()

	storedGithubConfig := &v33.GithubConfig{}
	if err := mapstructure.Decode(storedGithubConfigMap, storedGithubConfig); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}

	objectMeta, err := utils.ObjectMetaFromUnstructureContent(storedGithubConfigMap)
	if err != nil {
		return nil, err
	}
	storedGithubConfig.ObjectMeta = *objectMeta

	if storedGithubConfig.ClientSecret != "" {
		data, err := auth.ReadFromSecretData(g.Secrets, storedGithubConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for k, v := range data {
			if strings.EqualFold(k, mclient.GithubConfigFieldClientSecret) {
				storedGithubConfig.ClientSecret = string(v)
			} else {
				if storedGithubConfig.AdditionalClientIDs == nil {
					storedGithubConfig.AdditionalClientIDs = map[string]string{}
				}
				storedGithubConfig.AdditionalClientIDs[k] = strings.TrimSpace(string(v))
			}
		}
	}

	return storedGithubConfig, nil
}
